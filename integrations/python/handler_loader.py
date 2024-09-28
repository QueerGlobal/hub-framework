import importlib.util
import sys
import os
import yaml

class DynamicHandlerImporter:
    def __init__(self, yaml_file_path):
        """
        Initializes the DynamicHandlerImporter with a path to a YAML file containing handler configurations.

        :param yaml_file_path: Path to the YAML file with handler configurations.
        """
        self.yaml_file_path = yaml_file_path

    def _load_module_from_path(self, module_path):
        """
        Dynamically load a module from a relative file path.

        :param module_path: Relative path to the module file.
        :return: The loaded module object or None if loading failed.
        """
        try:
            module_name = os.path.splitext(os.path.basename(module_path))[0]
            spec = importlib.util.spec_from_file_location(module_name, module_path)
            module = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(module)
            return module
        except FileNotFoundError as e:
            print(f"Error: Module file {module_path} not found.")
        except Exception as e:
            print(f"Error loading module from {module_path}: {e}")
        return None

    def load_handlers(self):
        """
        Loads the handlers specified in the YAML file.

        :return: A dictionary of handlers with unique_name as key and handler function as value.
        """
        handlers = {}
        try:
            with open(self.yaml_file_path, 'r') as file:
                handler_configs = yaml.safe_load(file)

            for config in handler_configs:
                unique_name = config.get('unique_name')
                handler_name = config.get('handler_name')
                handler_path = config.get('handler_path')

                if not all([unique_name, handler_name, handler_path]):
                    print(f"Error: Invalid configuration for handler {unique_name}")
                    continue

                module_path = os.path.join(handler_path, f"{handler_name}.py")
                module = self._load_module_from_path(module_path)
                
                if module:
                    try:
                        handler = getattr(module, handler_name)
                        handlers[unique_name] = handler
                        print(f"Successfully loaded handler '{unique_name}' from '{module_path}'")
                    except AttributeError:
                        print(f"Error: Handler '{handler_name}' not found in module '{module_path}'")

        except FileNotFoundError:
            print(f"Error: YAML file {self.yaml_file_path} not found.")
        except yaml.YAMLError as e:
            print(f"Error parsing YAML file: {e}")
        except Exception as e:
            print(f"Unexpected error: {e}")

        return handlers

# Example Usage:

if __name__ == "__main__":
    yaml_file_path = 'path/to/your/handlers.yaml'
    importer = DynamicHandlerImporter(yaml_file_path=yaml_file_path)
    loaded_handlers = importer.load_handlers()

    # Accessing and using a handler
    if 'unique_handler_name' in loaded_handlers:
        handler = loaded_handlers['unique_handler_name']
        request = {'data': 'some request data'}
        handler(request)  # Assuming the handler takes a 'request' as a parameter
    else:
        print("Handler not found")