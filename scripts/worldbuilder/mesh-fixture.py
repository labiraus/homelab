#!/usr/bin/env python3
"""Synthetic HTTP endpoints/clients for the disposable mesh policy test only."""
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import socket
import ssl
import sys
import threading
from urllib.parse import urlsplit


def serve(role, ports):
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            path = urlsplit(self.path).path
            status = 200
            if role == 'external' and path == '/api/mcp-oauth/check':
                status = 200 if self.headers.get('Authorization') == 'Bearer synthetic-native-client' else 401
            body = json.dumps({'fixture': 'worldbuilder-mesh', 'server': role, 'path': path,
                               'email': self.headers.get('X-Auth-Request-Email'),
                               'forwardedCertificate': self.headers.get('X-Forwarded-Client-Cert')}).encode()
            self.send_response(status)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Content-Length', str(len(body)))
            if role == 'external' and path == '/api/mcp-oauth/check' and status == 200:
                self.send_header('X-Auth-Request-Email', 'member@example.invalid')
                self.send_header('X-Auth-Request-User', 'synthetic-member')
            self.end_headers(); self.wfile.write(body)

        do_POST = do_GET
        do_OPTIONS = do_GET

        def log_message(self, *args): pass

    servers = [ThreadingHTTPServer(('0.0.0.0', p), Handler) for p in ports]
    for server in servers[:-1]: threading.Thread(target=server.serve_forever, daemon=True).start()
    servers[-1].serve_forever()


def request(config):
    connection = None
    try:
        if config.get('certificate'):
            context = ssl.create_default_context(cadata=config['certificate'])
            connection = http.client.HTTPSConnection(config['hostname'], context=context, timeout=3)
            # Explicit fixture IP transport while retaining certificate/hostname
            # validation. Never resolve or connect to the real public hostname.
            connection.sock = context.wrap_socket(socket.create_connection((config['host'], config['port']), 3),
                                                 server_hostname=config['hostname'])
        else:
            connection = http.client.HTTPConnection(config['host'], config['port'], timeout=3)
        connection.request(config.get('method', 'GET'), config['path'], headers=config.get('headers', {}))
        response = connection.getresponse(); body = response.read(65537)
        if len(body) > 65536: raise ValueError('Fixture response exceeds bound')
        return {'status': response.status, 'body': body.decode('utf-8', errors='strict')}
    except (OSError, http.client.HTTPException) as error:
        return {'status': 0, 'error': type(error).__name__}
    finally:
        if connection: connection.close()


if __name__ == '__main__':
    if sys.argv[1] == 'serve': serve(sys.argv[2], [int(p) for p in sys.argv[3:]])
    elif sys.argv[1] == 'request': print(json.dumps(request(json.load(sys.stdin))))
    else: raise ValueError('Unknown fixture operation')
