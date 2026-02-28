import unittest
import json
from unittest.mock import patch
from app import app

class AppTestCase(unittest.TestCase):
    def setUp(self):
        self.app = app.test_client()
        self.app.testing = True

    def test_healthx(self):
        response = self.app.get('/liveness')
        self.assertEqual(response.status_code, 200)

    def test_healthz(self):
        response = self.app.get('/readiness')
        self.assertEqual(response.status_code, 200)

if __name__ == '__main__':
    unittest.main()