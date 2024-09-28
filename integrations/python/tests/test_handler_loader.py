import unittest
import shutil
import tempfile
import os
from handler_loader import DynamicHandlerImporter

class TestDynamicHandlerImporter(unittest.TestCase):
    def setUp(self):
        # Create a temporary YAML file
        self.temp_dir = tempfile.mkdtemp()
        self.yaml_content = """
        - unique_name: test_handler
          handler_name: test_function
          handler_path: {0}
        """.format(self.temp_dir)
        self.yaml_file = os.path.join(self.temp_dir, 'test_handlers.yaml')
        with open(self.yaml_file, 'w') as f:
            f.write(self.yaml_content)

        # Create a temporary Python file with a test function
        self.py_content = """
def test_function(request):
    return "Test function called with " + str(request)
"""
        self.py_file = os.path.join(self.temp_dir, 'test_function.py')
        with open(self.py_file, 'w') as f:
            f.write(self.py_content)

    def test_load_handlers(self):
        importer = DynamicHandlerImporter(self.yaml_file)
        handlers = importer.load_handlers()

        self.assertIn('test_handler', handlers)
        self.assertTrue(callable(handlers['test_handler']))
        
        # Test the loaded handler
        result = handlers['test_handler']({'test': 'data'})
        self.assertEqual(result, "Test function called with {'test': 'data'}")

    def tearDown(self):
        # Clean up temporary files
        shutil.rmtree(self.temp_dir, ignore_errors=True)

if __name__ == '__main__':
    unittest.main()