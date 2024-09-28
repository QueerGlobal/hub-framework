from flask import Flask, request, jsonify
from handler_loader import DynamicHandlerImporter
from utils import find_project_root
import json
import jsonschema
from jsonschema import RefResolver
import os

def create_app(handlers):
    app = Flask(__name__)

    project_root = find_project_root()
    os.chdir(project_root)

    # Load the JSON schema
    with open('api/schema/request.schema.json', 'r') as schema_file:
        schema = json.load(schema_file)

    # Create a resolver for the schema
    resolver = RefResolver.from_schema(schema)

    @app.route('/apply/<handler_name>', methods=['GET', 'POST', 'PUT'])
    def apply_handler(handler_name):
        # Get the JSON data from the request
        data = request.json

        # Validate the input structure
        if not isinstance(data, dict) or 'Handler' not in data or 'Request' not in data:
            return jsonify({"error": "Invalid input structure"}), 400

        # Validate the handler name
        if data['Handler'] != handler_name:
            return jsonify({"error": "Handler name mismatch"}), 400

        # Validate the request against the JSON schema
        try:
            jsonschema.validate(
                instance=data['Request'],
                schema=schema['definitions']['ServiceRequest'],
                resolver=resolver
            )
        except jsonschema.exceptions.ValidationError as validation_error:
            app.logger.error(f"Schema validation error: {str(validation_error)}")
            return jsonify({"error": f"Schema validation error: {str(validation_error)}"}), 400
        except Exception as e:
            app.logger.error(f"Unexpected error during schema validation: {str(e)}")
            return jsonify({"error": "Internal server error during request validation"}), 500

        # Check if the handler exists
        if handler_name not in handlers:
            return jsonify({"error": f"Handler '{handler_name}' not found"}), 404

        # Call the handler
        try:
            result = handlers[handler_name](data['Request'])
            return jsonify(result), 200
        except Exception as e:
            app.logger.error(f"Handler execution error: {str(e)}")
            return jsonify({"error": f"Handler execution error: {str(e)}"}), 500


    @app.route('/check/<handler_name>', methods=['GET'])
    def check_handler(handler_name):
        if handler_name in handlers:
            return jsonify({
                "status": "available",
                "handler": handler_name
            }), 200
        else:
            return jsonify({
                "status": "not_found",
                "handler": handler_name
            }), 404

    return app

if __name__ == '__main__':
    # For demonstration purposes, we'll create a dummy handlers map here
    dummy_handlers = {
        'test_handler': lambda x: {'message': f'Handled request with data: {x}'}
    }
    app = create_app(dummy_handlers)
    app.run(debug=True)
    
