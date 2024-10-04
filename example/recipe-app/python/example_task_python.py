import json
from typing import Dict, Any




def NewExampleTaskPython(**kwargs) -> "ExampleTaskPython":
    return ExampleTaskPython()


class ExampleTaskPython:
    def __init__(self, **kwargs):
        self.message = kwargs.get('message', 'a little extra love')

    def name(self) -> str:
        return "exampleTaskPython"

    def apply(self, context: Dict[str, Any], request: Dict[str, Any]) -> None:
        # Read the body from the ServiceRequest
        if not request or 'body' not in request or not request['body']:
            raise ValueError("Invalid request or empty body")

        # Apply the changes using the execute method
        result = self.execute(request['body'])

        # Update the request body with the result
        request['body'] = result

    def execute(self, input_data: bytes) -> bytes:
        # Deserialize recipe
        try:
            recipe = json.loads(input_data.decode('utf-8'))
        except json.JSONDecodeError as e:
            raise ValueError(f"Failed to unmarshal recipe: {str(e)}")

        # Add "a little extra love" as an ingredient
        if 'ingredients' not in recipe:
            recipe['ingredients'] = []
        recipe['ingredients'].append("a little extra love")

        # Serialize the updated recipe
        try:
            updated_recipe_json = json.dumps(recipe).encode('utf-8')
        except Exception as e:
            raise ValueError(f"Failed to marshal updated recipe: {str(e)}")

        return updated_recipe_json

# Example usage:
if __name__ == "__main__":
    task = ExampleTaskPython()
    sample_request = {
        'body': json.dumps({
            'name': 'Chocolate Cake',
            'ingredients': ['flour', 'sugar', 'cocoa powder'],
            'steps': ['Mix dry ingredients', 'Add wet ingredients', 'Bake'],
            'comments': [{'text': 'Delicious!'}]
        }).encode('utf-8')
    }

    task.apply({}, sample_request)
    print(json.loads(sample_request['body']))