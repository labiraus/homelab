#!/usr/bin/env python3
"""Synthetic provider and IMDSv2 endpoints for an owned offline fixture only."""
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import pathlib
import ssl
import sys
import threading
import time

role = sys.argv[1]
generation = 0

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        # Synthetic paths/status only; no credential values or request payloads.
        print(role, self.command, self.path, args[1:] if len(args) > 1 else '', flush=True)

    def reply(self, status, body):
        raw = body.encode() if isinstance(body, str) else json.dumps(body).encode()
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(raw)))
        if self.command == 'PUT':
            self.send_header('X-Aws-Ec2-Metadata-Token-Ttl-Seconds', '21600')
        self.end_headers()
        self.wfile.write(raw)

    def do_PUT(self):
        self.reply(200, 'offline-imds-token')

    def do_GET(self):
        global generation
        if role != 'credentials':
            self.reply(200, {'ready': True})
        elif pathlib.Path('/tmp/deny').exists():
            self.reply(503, {'error': 'offline credential refusal'})
        elif self.headers.get('X-aws-ec2-metadata-token') != 'offline-imds-token':
            self.reply(401, {})
        elif self.path.rstrip('/').endswith('/security-credentials'):
            self.reply(200, 'offline-role')
        else:
            generation += 1
            self.reply(200, {'Code': 'Success', 'Type': 'AWS-HMAC', 'AccessKeyId': 'OFFLINEONLY',
                            'SecretAccessKey': 'offline-not-an-aws-key', 'Token': 'offline-session-' + str(generation) + '-' + str(int(time.time()) + 3),
                            'Expiration': (datetime.now(timezone.utc) + timedelta(seconds=2)).isoformat()})

    def do_POST(self):
        # Envoy's external processor can remove Content-Length after translation.
        if self.headers.get('Transfer-Encoding', '').lower() == 'chunked':
            raw = bytearray()
            while True:
                size = int(self.rfile.readline().split(b';', 1)[0], 16)
                if size == 0:
                    while self.rfile.readline() != b'\r\n':
                        pass
                    break
                raw.extend(self.rfile.read(size))
                assert self.rfile.read(2) == b'\r\n'
            body = json.loads(raw)
        else:
            body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        if self.path.endswith('/converse'):
            token = self.headers.get('X-Amz-Security-Token', '')
            if not self.headers.get('Authorization', '').startswith('AWS4-HMAC-SHA256 ') or not token.startswith('offline-session-'):
                return self.reply(403, {})
            if int(token.rsplit('-', 1)[1]) <= time.time():
                return self.reply(403, {'message': 'ExpiredToken'})
            content, reason = [{'text': token}], 'end_turn'
            if body.get('toolConfig'):
                if body['toolConfig'].get('toolChoice') != {'any': {}}:
                    return self.reply(400, {'message': 'required tool translation failed'})
                name = body['toolConfig']['tools'][0]['toolSpec']['name']
                content, reason = [{'toolUse': {'toolUseId': 'offline-tool', 'name': name, 'input': {'answer': 'blue'}}}], 'tool_use'
            self.reply(200, {'output': {'message': {'role': 'assistant', 'content': content}},
                             'stopReason': reason, 'usage': {'inputTokens': 10, 'outputTokens': 2, 'totalTokens': 12}, 'metrics': {'latencyMs': 1}})
        else:
            self.reply(200, {'id': 'offline', 'object': 'chat.completion', 'model': 'offline',
                             'choices': [{'index': 0, 'message': {'role': 'assistant', 'content': 'blue'}, 'finish_reason': 'stop'}],
                             'usage': {'prompt_tokens': 10, 'completion_tokens': 2, 'total_tokens': 12}})

if role == 'credentials':
    ThreadingHTTPServer(('127.0.0.1', 9911), Handler).serve_forever()
else:
    plain = ThreadingHTTPServer(('0.0.0.0', 8000), Handler)
    threading.Thread(target=plain.serve_forever, daemon=True).start()
    tls = ThreadingHTTPServer(('0.0.0.0', 8443), Handler)
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.load_cert_chain('/identity/tls.crt', '/identity/tls.key')
    tls.socket = context.wrap_socket(tls.socket, server_side=True)
    tls.serve_forever()
