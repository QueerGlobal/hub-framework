import os

def find_project_root(current_path=None):
    """
    Find the project root by looking for a .git directory or a specific file that indicates the root.
    """
    if current_path is None:
        current_path = os.getcwd()
    
    while True:
        if os.path.exists(os.path.join(current_path, '.git')):
            return current_path
        parent_path = os.path.dirname(current_path)
        if parent_path == current_path:
            raise FileNotFoundError("Project root not found. Are you inside the project directory?")
        current_path = parent_path