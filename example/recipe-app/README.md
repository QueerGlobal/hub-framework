# Example Application: Recipes

## Concept

This is an example application showing how the Hub Framework 
works using a simple domain, a recipe application. 

This application consists of two root aggregates, a recipe and a chef, 
and shows some basic tasks that can be applied to incoming requests
and outgoing responses.

## Layout

### Aggregates

In the /aggregates directory, you will find yaml files specifying 
the our two aggregates. 

In recipe.yaml, we see an example specification for an aggregate. 

The specification specifies the name, the name of the API this 
aggregate is a part of, the name of the schema the user has provided for the aggregate, 
and whether it is available via the application's public port. 

The spec also identifies the target (destination for incoming data) and workflows 
consisting of a set of inbound and outbound processing steps. 

```yaml
apiVersion: v1
specType: Aggregate
spec:
  name: recipe
  apiName: recipeApp
  isPublic: true
  schema: Recipe
  target:
    name: persistRecipe
    type: Badger
  workflow:
    - methods: ["POST", "PUT", "DELETE"]
      inbound:
        - name: requestLogger
          ref: builtin.RequestLogger
          precedence: 1
      outbound:
        - name: searchRegistrar
          description: "register recipe for search"
          type: workflowStep
          ref: global.SearchRegistrar
          executionType: async
          config:
            serviceName: "recipe"
            key: headers.SearchKey
            content: body
        - name: responseCodeLogger
          description: "log response code"
          ref: builtin.ResponseCodeLogger
          config:
            logAtLevel: INFO
        - name: responseBodyLogger
          description: "log response body"
          ref: builtin.ResponseCodeLogger
          config:
            logAtLevel: DEBUG
    - methods: ["GET"]
      inbound:
        - name: requestLogger
          ref: builtin.RequestLogger
          precedence: 1
      outbound:
        - name: responseCodeLogger
          description: "log response code"
          ref: builtin.ResponseCodeLogger
          config:
            logAtLevel: INFO
        - name: responseBodyLogger
          description: "log response body"
          ref: builtin.ResponseCodeLogger
          config:
            logAtLevel: DEBUG

```

### Schemas 

the /schemas directory contains a set of schema files provided by the user, which specify the fields of 
each of the entities and aggregates the application will deal with. 

It also contains a schemas.yaml file, where a name, version, and file location for each schema is specified.

```yaml
apiVersion: v1
specType: Schemas
spec:
  compatibility: "backward"
  schemas:
    - name: Person
      version: v0.0.1
      file: "{$project-root}/schemas/person-aggregate.schema.json"
    - name: Recipe
      version: v0.0.1
      filename: "{$project-root}/schemas/recipe-aggregate.schema.json"

```

Currently schemas can be provided in the [JSON Schema] (https://json-schema.org/) format. 
We will consider adding additional schema specification formats, such as protobuf or Avro
in the future. 

Below is an example schema for the recipe aggregate, containing recipe-specific fields
and all other entities composing the recipe aggregate (in this case ingredients and comments.)

```json
{
  "$id": "https://example.com/schemas/recipe/aggregate",
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "Recipe",
  "type": "object",
  "properties": {
    "title": {
      "type": "string",
      "description": "The recipe title"
    },
    "ownerId": {
      "description": "The ID of the person who created this recipe.",
      "type": "number",
      "minimum": 0
    },
    "ingredients": {
      "type": "array",
      "items": {
        "type": {"$ref" : "/recipe/ingredient"}
      }
    },
    "comments": {
      "type": "array",
      "items": {
        "type": {"$ref" : "/recipe/comment"}
      }
    }
  }
}
```

### Tasks

In the /tasks directory we have a set of yaml files
each containing a spec identifying one or more tasks that can 
be applied to a request or response as a part of an aggregate's
workflow, along with its configuration. 

Below is an example specification:

```yaml
apiVersion: v1
specType: Tasks
spec:
  namespace: recipe
  tasks:
    - name: RecipeValidator
      description: "validates fields on the recipe aggregate"
      type: FieldValidator
      executionType: Synchronous
      mustFinish: true
      onError: LogAndFail
      config:
        - schema: Recipe
          validations:
            - name: "validate owner exists"
              path: $.ownerId
              condition: NotNull
            - name: "validate ingredients not empty"
              path: $.ingredients
              condition: NotEmpty
            - name: "validate ingredient units of measurement are present"
              field: $.ingredients[*].unitsOfMeasurement
              condition: NotNull
            
```

### Application

/hub.yaml contains basic configuration for the application,
including public and private ports, application version, and
all APIs and the aggregates within.

Below is an example configuration: 

```yaml
apiVersion: v1
specType: Hub
spec:
  applicationName: "recipe-app"
  applicationVersion: "v0.0.1"
  publicPort: 8081
  privatePort: 8082
  apis:
    - name: recipes
      aggregates:
        - name: chef
        - name: recipe
```
