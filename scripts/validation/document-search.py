#!/usr/bin/env python3
"""Validate real document inference/retrieval using explicitly owned synthetic resources.

Requires processor's installed development dependencies and Go. The supplied
OpenSearch endpoint/model are explicit; this is an operator acceptance check,
not a substitute for an offline fixture. No source document is read or queued.
"""
import argparse
import json
import math
import os
from pathlib import Path
import subprocess
import tempfile
import urllib.error
import urllib.parse
import urllib.request
import uuid

ROOT = Path(__file__).resolve().parents[2]


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


def assert_text_coverage(source, rows):
    # Retain possible positions for repeated text. Greedily choosing the last
    # match can reject valid overlap between the character and word stages.
    frontier = [(0, 0)]
    for row in rows:
        text = row['text']
        assert text and len(text.encode('utf-16-le')) // 2 <= 480, 'invalid Unicode or oversized chunk'
        following = []
        index, covered = 0, -1
        start = source.find(text, frontier[0][0])
        while 0 <= start <= frontier[-1][1]:
            while index < len(frontier) and frontier[index][0] <= start:
                covered = max(covered, frontier[index][1])
                index += 1
            if start <= covered:
                end = max(covered, start + len(text))
                if not following or end > following[-1][1]:
                    following.append((start, end))
            start = source.find(text, start + 1)
        assert following, 'changed, missing or out-of-order text'
        frontier = following
    assert frontier[-1][1] == len(source), 'source tail missing'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--base-url', required=True)
    parser.add_argument('--model-id')
    parser.add_argument('--chunking-only', action='store_true', help='Only simulate text integrity; does not establish inference or retrieval')
    args = parser.parse_args()
    if not args.chunking_only and not args.model_id:
        parser.error('--model-id is required for inference and retrieval acceptance')
    address = urllib.parse.urlsplit(args.base_url)
    if address.scheme not in ('http', 'https') or not address.hostname or address.username or address.password or address.query or address.fragment or address.path not in ('', '/'):
        parser.error('expected an explicit HTTP(S) origin without credentials or a path')
    base = args.base_url.rstrip('/')
    owner = str(uuid.uuid4())
    name = 'recovery-document-search-' + owner
    description = 'Owned synthetic document acceptance ' + owner
    opener = urllib.request.build_opener(NoRedirect)

    def request(method, path, body=None):
        payload = None if body is None else json.dumps(body).encode()
        req = urllib.request.Request(base + path, method=method, data=payload,
                                     headers={'Content-Type': 'application/json'})
        try:
            with opener.open(req, timeout=120) as response:
                return json.load(response)
        except urllib.error.HTTPError as error:
            # Only synthetic data is sent; never print connection credentials.
            raise RuntimeError(f'{method} {path}: HTTP {error.code}: {error.read(2000).decode()}') from error

    config = {'openSearchModelId': args.model_id or 'unused', 'openSearchVectorDimensions': 1024,
              'openSearchChunkTokenLimit': 384, 'openSearchChunkCharLimit': 480,
              'openSearchChunkOverlapRate': 0.15}
    node = """
import fs from 'node:fs';
import {buildIndexPayload,buildIngestPipelinePayload,buildSearchPipelinePayload} from './src/opensearch.ts';
const config=JSON.parse(fs.readFileSync(0,'utf8'));
console.log(JSON.stringify({index:buildIndexPayload(config),ingest:buildIngestPipelinePayload(config),search:buildSearchPipelinePayload(config)}));
"""
    payloads = json.loads(subprocess.check_output(
        ['node', '--import', 'tsx', '--input-type=module', '-e', node],
        cwd=ROOT / 'apps/processor', input=json.dumps(config).encode()))
    payloads['index']['settings']['index']['number_of_shards'] = 1
    payloads['index']['settings']['index']['number_of_replicas'] = 0
    payloads['index']['mappings']['_meta'] = {'acceptanceOwner': owner}
    for kind in ['ingest', 'search']:
        payloads[kind]['description'] = description
    # Real Painless/analyzer execution, including the old whole-document token
    # ceiling and UTF-16 boundary failure. No model is invoked for these cases.
    chunk_texts = [''.join(f'token{i:05d} ' for i in range(41000)) + ' FINAL_SENTINEL',
                   'a' * 479 + '😀 tail', ('😀🌍 漢字 café\n' * 80) + ' FINAL_SENTINEL',
                   '漢字' * 400 + ' FINAL_SENTINEL']
    chunk_pipeline = {'processors': payloads['ingest']['processors'][:3]}
    for overlap in (0, 0.15, 0.5):
        chunk_pipeline['processors'][0]['script']['params']['overlap_rate'] = overlap
        simulation = request('POST', '/_ingest/pipeline/_simulate', {
            'pipeline': chunk_pipeline, 'docs': [{'_source': {'body': text}} for text in chunk_texts]})
        for original, result in zip(chunk_texts, simulation['docs']):
            assert 'error' not in result, json.dumps(result)
            assert_text_coverage(original, result['doc']['_source']['passage_chunk'])
    chunk_pipeline['processors'][0]['script']['params']['overlap_rate'] = config['openSearchChunkOverlapRate']
    if args.chunking_only:
        print(json.dumps({'status': 'PASS', 'chunkingCases': len(chunk_texts) * 3, 'inference': 'not_tested', 'retrieval': 'not_tested'}))
        return
    expected = 'A quoted "word" and <angle> & ampersand, Unicode café 漢字, a slash \\ and newline\nremain exact.'
    docs = [
        {'documentId': 'fixture-target', 'objectKey': 'fixture/target.txt', 'status': 'processed',
         'metadata': {'scope': 'target'}, 'body': expected},
        {'documentId': 'fixture-other', 'objectKey': 'elsewhere/other.txt', 'status': 'processed',
         'metadata': {'scope': 'other'}, 'body': expected},
        {'documentId': 'fixture-long', 'objectKey': 'fixture/long.txt', 'status': 'processed',
         'metadata': {'scope': 'long'},
         'body': ('Electroencephalographically counterrevolutionaries, "quoted" text and backslashes \\ stay intact.\n' * 30) + ' FINAL_SENTINEL'},
    ]
    created = []
    try:
        state = request('GET', '/_plugins/_ml/models/' + urllib.parse.quote(args.model_id, safe=''))
        assert state['model_state'] == 'DEPLOYED', 'model is not ready for worker startup'
        model = request('POST', '/_plugins/_ml/models/' + urllib.parse.quote(args.model_id, safe='') + '/_predict',
                        {'parameters': {'input': 'An owned synthetic embedding acceptance request.'}})
        response = model['inference_results'][0]['output'][0]['dataAsMap']
        vector = response['data'][0]['embedding']
        assert len(vector) == 1024 and all(math.isfinite(x) for x in vector), 'invalid model prediction'
        for kind, path in [('index', '/' + name), ('ingest', '/_ingest/pipeline/' + name), ('search', '/_search/pipeline/' + name)]:
            request('PUT', path, payloads[kind]); created.append((kind, path))
        simulation = request('POST', '/_ingest/pipeline/' + name + '/_simulate', {'docs': [{'_source': d} for d in docs]})
        chunks = []
        for original, result in zip(docs, simulation['docs']):
            assert 'error' not in result, json.dumps(result)
            source = result['doc']['_source']; assert 'body' not in source
            rows = source['passage_chunk']; assert rows
            assert_text_coverage(original['body'], rows)
            for chunk in rows:
                assert isinstance(chunk['text'], str) and len(chunk['text']) <= 480
                vector = chunk['embedding']
                assert len(vector) == 1024 and all(math.isfinite(x) for x in vector)
                assert 0.99 < math.sqrt(sum(x*x for x in vector)) < 1.01
            chunks.append(rows)
        assert chunks[0][0]['text'] == expected and chunks[1][0]['text'] == expected, 'source text changed'
        assert len(chunks[2]) > 1 and 'FINAL_SENTINEL' in chunks[2][-1]['text'], 'long document lost its tail'
        for document in docs:
            request('PUT', '/' + name + '/_doc/' + document['documentId'] + '?pipeline=' + name + '&refresh=true', document)
        # Generate real application queries, rather than reimplementing its filters.
        queries = [{'Query': 'quoted', 'Limit': 5},
                   {'Query': 'quoted', 'DocumentID': 'fixture-target'},
                   {'Query': 'quoted', 'Prefix': 'elsewhere/'},
                   {'Query': 'quoted', 'Metadata': {'scope': 'target'}},
                   {'Query': 'quoted', 'DocumentID': 'fixture-target', 'Metadata': {'scope': 'other'}}]
        go = '''package main
import("encoding/json";"os";"pkg/opensearchrag")
func main(){var queries []opensearchrag.SearchRequest;if err:=json.NewDecoder(os.Stdin).Decode(&queries);err!=nil{panic(err)};var bodies []any;for _,q:=range queries{bodies=append(bodies,opensearchrag.BuildSearchBody(q))};if err:=json.NewEncoder(os.Stdout).Encode(bodies);err!=nil{panic(err)}}
'''
        with tempfile.TemporaryDirectory(prefix='homelab-document-query-') as temp:
            path = Path(temp) / 'main.go'; path.write_text(go)
            bodies = json.loads(subprocess.check_output(['go', 'run', str(path)], cwd=ROOT / 'apps/pkg/opensearchrag', input=json.dumps(queries).encode(), env=os.environ))
        expected_ids = [{'fixture-target', 'fixture-other', 'fixture-long'}, {'fixture-target'}, {'fixture-other'}, {'fixture-target'}, set()]
        for body, wanted in zip(bodies, expected_ids):
            result = request('POST', '/' + name + '/_search', body)
            assert result['_shards']['failed'] == 0
            actual = {hit['_id'] for hit in result['hits']['hits']}
            assert actual == wanted, f'application filter mismatch: {actual} != {wanted}'
        semantic = {'query': {'template': {'nested': {'path': 'passage_chunk', 'query': {'knn': {'passage_chunk.embedding': {'vector': '${ext.ml_inference.params.query_embedding}', 'k': 3}}}}}}, 'ext': {'ml_inference': {'params': {'query_text': 'quoted word'}}}, 'size': 3, '_source': False}
        found = request('POST', '/' + name + '/_search?search_pipeline=' + name, semantic)
        assert found['_shards']['failed'] == 0 and len(found['hits']['hits']) == 3, 'semantic pipeline did not execute'
        report = {'status': 'PASS', 'dimensions': 1024, 'syntheticDocuments': len(docs),
                  'chunkCounts': [len(c) for c in chunks], 'textIntegrity': True,
                  'applicationFilterCases': len(bodies), 'semanticPipeline': True}
    finally:
        for kind, path in reversed(created):
            value = request('GET', path)[name]
            actual = value['mappings']['_meta']['acceptanceOwner'] if kind == 'index' else value['description']
            assert actual == (owner if kind == 'index' else description), 'resource ownership changed; refusing cleanup'
            request('DELETE', path)
    print(json.dumps(report))


if __name__ == '__main__':
    main()
