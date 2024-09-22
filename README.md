# The Hub Application Framework

Building great software is no easy task, and it takes a village. 
At Queer Global we rely on like-minded engineers to help us build a 
great software, and we want to make that kind of collaboration as
easy as possible. 

Even while tools like AI give us new and exciting capabilities, the challenges 
of building software that is easy for community members to contributo to remain.

We need to manage complexity, ensure quality, and make it easy to collaborate with one another. 

This project aims to help us at QG do that, and we want to share it with you: 
- It makes it easy to collaborate by providing a common architecture and the ability to choose the language you want to code in. 
- It makes it easy to build software by providing a clear model for managing data and business logic. 
- It provides a platform for solving the hard problems inherent in distributed systems. 

:construction: This project is under active construction and currently requires
some work before it will reach its full potential. We're excited about what we're
building here and we welcome your help bringing something great to the community! :construction:
 

## Background

This project originated because we rely on help from our friends. We 
want to make it as easy as possible for an engineer to make a meaningful
contribution with whatever time they have. 

We prefer not to be picky about what language or technology they want 
to use to help us out. 

We also need an architecture and data model that is easy for 
newcomers to understand, and ideally one that makes it easy for 
frontend engineers to create the backend storage and entities they 
need. 

With these goals in mind, we decided building a framework we could reuse
as we add new components, and which would supply a strong, clear programming
model would be best for our needs. 

Enter the Hub. 

This project aims to create an application framework for the 
cloud-based, distributed world we live in. We aim to take notes
from great web application frameworks like Ruby on Rails, and 
Django, and to provide a simple model for representing
data. 

We want to simplify the way we represent and interact with 
data in our applicaiton. Software projects often start of cleanly, 
but eventually approach a "ball of mud" state where their internal 
complexity grows exponentially, and see progress grind to a halt 
as developers struggle to understand a tangled web of internal 
dependencies before they can make meaningful contributions. 


## Design

We believe that most, if not all interactions within a system 
can be represented by CRUD (Create, Read, Update, Delete) operations 
on a few key objects in the system, or aggregates. 

In short, if you are building a system, that manges recipes, you should
have a "thing" called a Recipe in your system. Everything that composes
a recipe (ingredients, instructions, etc.) should be nested inside. And
when you change the recipe, the general process should be that you find
the recipe, update it, and save it. 

We also know that there are often other things we want to do with our 
data other than CRUD. FWe may need to check whether a user has a 
subscription, save changes to the object to a search index, or notify
other services of some change. 

While extremely important, these additional operations do not need to be 
part of the domain model, and are often the source of a lot of the comlexity
in software projects. 

So instead of mashing them into our object-updating process, we separate 
those concerns by applying them as a series of workflow operations 
chained together and applied to our incoming and outgoing data. 

Each workflow step can modify the incoming or outgoing data, or performe
operations with side effects, with the constraint that they must preserve
the schema (the type and structure)of the incoming data. 

The most exciting thing about this approach is that we distribute those 
workflow tasks to other machines, services, or cloud functions, and we can 
write them in any language we choose (see the roadmap section below
for language support plans). 

This modular approach to application development also makes it easy to 
swap out and iterate on individual parts of the system without affecting
the core data models or the rest of the application. 


## AI 

Similar to humans, AI agents have a limited amount of cognitive space they
can contribute to a problem at one time. This is known as a context window. 

These agents work best when the information they need to consider 
when working on a task is clear and limited in scope. (In our opinion, this 
is true of people too!)

For this reason, keeping CRUD operations separate from other concerns, and 
keeping our workflow tasks isolated from one another not only makes 
understanding our systems easier for people, but it also creates a fantastic 
environment for developing with AI.


## Usage

When working with this framework, the developer can specify 
schemas for a set of aggregates (the key aggregates, 
or domain-related groups of objects) in their domains using JSON-schema.

They can also specify handlers, workflows, and target desitinations (data stores)
for their aggregates. 

Please look at our [example app](example/README.md) for a 
demonstration of how this framework could be used in a simple
use case. 

These services supply handlers for each http method, specifying  
incoming and outgoing workflows (series of 
tasks or transformations that can be applied to incoming requests and 
outgoing responses) and a target (the destination where the
incoming data will be stored)

:construction: note: The code generation aspects of this 
application are still in development. Below is a description 
of how installation and usage will look :construction:

To install the application, run the following from the hub 
project root:

```bash
./scripts/install.sh 
```

To start a new application, execute following command: 

```bash
hub new "my-application-name"
```

This will create a new application directory with default yaml files.

After changing yaml spec files, adding a new schema or changing an
existing one, one can run the following command from
within the application directory: 

```bash
hub build 
```
This will build the application, and also regenerate any 
generated files that are a part of the application. 

You can the application by running the following from
the project directory:

```bash
go main.go 
```

## Testing

The tests for the majority hub framework itself are written in 
golang, and can be run with the following command from the project
root. 

```bash
go test ./...
```

## Roadmap

As noted above, the reasons for building this project are
a simplified programming model, easy code generation
and the ability to contribute in multiple languages. 

Our highest priority tasks right now is building out the 
our adapters to other languages. The languages we are
targeting first are:

- Python
- Node.js
- Java
- Ruby

If there is another language you'd like to contribute in, 
please feel free to open a discussion about adding support. 

Other key priorities include: 
- Code generation from the yaml spec files
- Adding additional builtin workflow tasks like:
    - Opentelemtry trace initiation
    - OpenTelemetry metrics collection
    - JWT Validation
    - Configurable Rate limiting
- Schema versioning / schema registory / migration support (eg enforcable backwards compatibility)
- Improved startup scripts and workflow task registration
- Improved project generation scripts
- Local development environment improvements
- In-place upgrade scripts


- We would eventually like to build some key distributed system primitives on top of this framework, too. Like: 
    - Lamport / Vector Clock based concurrency control
    - Distributed Locks
    - Distributed Tracing
    - Updatable Configuration
    - Distributed Secret Management
    - Distributed Logging
    - Distributed Metrics
    

## Contributing

Please see our orgnazations documentation page [here](https://github.com/QueerGlobal/qg-docs) for more information on us and on how to contribute to this project. 

We will be holding ongoing development conversations as a part of this repository. 

Pull requests are welcome. For major changes, please open an issue first to discuss what you would like to change.

Thank you for visiting us!

## License
[Apache2.0](https://www.apache.org/licenses/LICENSE-2.0)
