import unittest
from unittest.mock import patch, MagicMock
import json
import os
from utils import find_project_root
from request_handler import create_app

# Change to project root directory
project_root = find_project_root()
os.chdir(project_root)

class TestRequestHandler(unittest.TestCase):
    def setUp(self):
        self.dummy_handlers = {
            'test_handler': lambda x: {'result': 'success', 'input': x}
        }
        self.app = create_app(self.dummy_handlers).test_client()

    def test_apply_handler_success(self):
        test_data = {
            "Handler": "test_handler",
            "Request": {
                "ApiName": "test_api",
                "ServiceName": "test_service",
                "Method": "POST",
                "URL": "http://example.com/test",
                "InternalPath": "/test",
                "RequestMeta": {
                    "Proto": "HTTP/1.1",
                    "ProtoMajor": 1,
                    "ProtoMinor": 1,
                    "ContentLength": 0,
                    "Host": "example.com",
                    "RemoteAddr": "127.0.0.1",
                    "RequestURI": "/test"
                }
            }
        }

        response = self.app.post('/apply/test_handler', json=test_data)
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json, {"result": "success", "input": test_data["Request"]})

    def test_apply_handler_invalid_input(self):
        test_data = {"InvalidKey": "InvalidValue"}
        response = self.app.post('/apply/test_handler', json=test_data)
        self.assertEqual(response.status_code, 400)
        self.assertIn("Invalid input structure", response.json['error'])

    def test_apply_handler_name_mismatch(self):
        test_data = {
            "Handler": "wrong_handler",
            "Request": {}
        }
        response = self.app.post('/apply/test_handler', json=test_data)
        self.assertEqual(response.status_code, 400)
        self.assertIn("Handler name mismatch", response.json['error'])

    def test_apply_handler_not_found(self):
        test_data = {
            "Handler": "non_existent_handler",
            "Request": {
                "ApiName": "test_api",
                "ServiceName": "test_service",
                "Method": "POST",
                "URL": "http://example.com/test",
                "InternalPath": "/test",
                "RequestMeta": {
                    "Proto": "HTTP/1.1",
                    "ProtoMajor": 1,
                    "ProtoMinor": 1,
                    "ContentLength": 0,
                    "Host": "example.com",
                    "RemoteAddr": "127.0.0.1",
                    "RequestURI": "/test"
                }
            }
        }
        response = self.app.post('/apply/non_existent_handler', json=test_data)
        self.assertEqual(response.status_code, 404)
        self.assertIn("Handler 'non_existent_handler' not found", response.json['error'])

    def test_check_handler_available(self):
        response = self.app.get('/check/test_handler')
        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.json, {"status": "available", "handler": "test_handler"})

    def test_check_handler_not_found(self):
        response = self.app.get('/check/non_existent_handler')
        self.assertEqual(response.status_code, 404)
        self.assertEqual(response.json, {"status": "not_found", "handler": "non_existent_handler"})

if __name__ == '__main__':
    unittest.main()