#!/usr/bin/env python3
"""Authenticated, explicit local maintenance and synthetic API measurement."""
import argparse
import concurrent.futures
import json
import os
import statistics
import subprocess
import time
import urllib.request
import uuid

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('action', choices=['status', 'drain', 'resume', 'evaluate', 'usage'])
    p.add_argument('--url', default='http://127.0.0.1:18080')
    p.add_argument('--model', default='local-default', choices=['local-default', 'bedrock-nova', 'bedrock-sonnet'])
    p.add_argument('--revision', default='')
    p.add_argument('--paid', action='store_true', help='Explicitly permit a paid model evaluation within the proxy budget')
    p.add_argument('--count', type=int, default=4)
    p.add_argument('--gpu-pod', help='Optional existing vLLM Pod name for read-only NVIDIA memory samples')
    p.add_argument('--namespace', default='homelab')
    a = p.parse_args()
    token = os.environ.get('INFERENCE_OPERATOR_TOKEN')
    if not token:
        p.error('INFERENCE_OPERATOR_TOKEN required; never pass tokens as command arguments')
    if a.model != 'local-default' and not a.paid:
        p.error('Cloud evaluations require --paid')
    if not 1 <= a.count <= 20:
        p.error('--count must be 1–20')
    def request(path, body=None):
        headers = {'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json', 'X-Request-ID': str(uuid.uuid4())}
        req = urllib.request.Request(a.url.rstrip('/') + path, headers=headers, data=None if body is None else json.dumps(body).encode())
        with urllib.request.urlopen(req, timeout=70) as response:
            return json.load(response)
    if a.action in ('drain', 'resume', 'status'):
        result = request('/admin/local', None if a.action == 'status' else {'drain': a.action == 'drain'})
        deadline = time.monotonic() + 90
        while a.action == 'drain' and result['active'] and time.monotonic() < deadline:
            time.sleep(1)
            result = request('/admin/local')
        if a.action == 'drain' and result['active']:
            raise RuntimeError('local inference did not drain within 90 seconds')
        print(json.dumps(result))
    elif a.action == 'usage':
        print(json.dumps(request('/v1/usage')))
    else:
        def run(index):
            body = {'model': a.model, 'max_tokens': 64, 'temperature': 0,
                    'messages': [{'role': 'user', 'content': 'The gate is blue. What colour is the gate? Answer briefly.'}]}
            if a.revision:
                body['profile_revision'] = a.revision
            start = time.monotonic()
            result = request('/v1/chat/completions', body)
            elapsed = time.monotonic() - start
            assert 'blue' in result['choices'][0]['message']['content'].lower(), 'synthetic quality check failed'
            return {'case': index, 'latency_seconds': elapsed, 'model': result['model'],
                    'usage': result.get('usage'), 'inference': result.get('inference')}
        def gpu():
            if not a.gpu_pod:
                return None
            return subprocess.check_output(['kubectl', '-n', a.namespace, 'exec', a.gpu_pod, '-c', 'vllm', '--', 'nvidia-smi', '--query-gpu=name,memory.used,memory.total', '--format=csv,noheader,nounits'], text=True, timeout=20).strip()
        before = gpu()
        start = time.monotonic()
        # Keep first/cold call separate; subsequent pairs measure supported concurrency.
        results = [run(0)]
        with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
            results += list(executor.map(run, range(1, a.count)))
        duration = time.monotonic() - start
        print(json.dumps({'gpu_memory_before': before, 'gpu_memory_after': gpu(), 'results': results, 'requests_per_second': a.count / duration,
                          'output_tokens_per_second': sum((r['usage'] or {}).get('completion_tokens') or 0 for r in results) / duration,
                          'median_latency_seconds': statistics.median(r['latency_seconds'] for r in results)}, indent=2))

if __name__ == '__main__':
    main()
