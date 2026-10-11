import json
import os
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer
from types import SimpleNamespace
from unittest.mock import patch
from urllib.error import HTTPError
from search_api import retrieve, validate_hits


class AdapterTests(unittest.TestCase):
    def test_contract_and_no_redirected_credentials(self):
        hit={"documentId":"source","chunkText":"Aster guards the bridge.","chunkIndex":2,"processingVersion":3,"citation":{"id":"s3://private/source#chunk-2","chunkIndex":2,"processingVersion":3}}
        received=[]
        class Handler(BaseHTTPRequestHandler):
            def log_message(self,*args): pass
            def do_POST(self):
                received.append((self.path,dict(self.headers),json.loads(self.rfile.read(int(self.headers["Content-Length"])))))
                if len(received)>1:
                    self.send_response(302);self.send_header("Location","http://127.0.0.1:1/leak");self.end_headers();return
                self.send_response(200);self.end_headers();self.wfile.write(json.dumps({"hits":[hit]}).encode())
        server=HTTPServer(("127.0.0.1",0),Handler)
        thread=threading.Thread(target=server.serve_forever,daemon=True);thread.start()
        try:
            case=SimpleNamespace(query="Who guards?",limit=1,document_id="source",prefix="campaign/",metadata={"world":"aster"})
            with patch.dict(os.environ,{"RAGAS_API_COOKIE":"offline-fixture-only"}):
                self.assertEqual(retrieve(f"http://127.0.0.1:{server.server_port}",case),[hit])
                with self.assertRaises(HTTPError):retrieve(f"http://127.0.0.1:{server.server_port}",case)
            self.assertEqual(received[0][0],"/api/documents/search")
            self.assertEqual(received[0][2]["metadata"],case.metadata)
            self.assertEqual(received[0][1]["Cookie"],"offline-fixture-only")
            self.assertNotIn("X-Forwarded-Email",received[0][1])
        finally:server.shutdown();thread.join();server.server_close()
        with self.assertRaises(ValueError):validate_hits({"hits":[{**hit,"processingVersion":4}]},1)
        with self.assertRaises(ValueError):retrieve("http://untrusted.test",case)
        with self.assertRaises(ValueError):validate_hits({"hits":[{**hit,"citation":None}]},1)

if __name__=="__main__":unittest.main()
