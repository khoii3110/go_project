# Technical Documentation

## REST with JSON
REST with JSON was chosen because it is easy to understand, widely supported, and ideal for service-to-service and client-to-server communication. Compared with GraphQL or gRPC, REST is easier to test manually, document, and consume from web or mobile clients. The trade-off is that REST can require multiple requests for related data and does not enforce strict contract typing at runtime. In this project, REST fits well because users can register, manage teams, and handle assets through clear endpoints with consistent JSON payloads.

## PostgreSQL
PostgreSQL was chosen over MongoDB because the project has strongly structured data and clear relationships between users, teams, memberships, sessions, and assets. Relational constraints, joins, transactions, and indexing make PostgreSQL a better match for this domain. The trade-off is that schema changes require migrations and careful planning, while MongoDB may feel more flexible early on. For this system, PostgreSQL improves data integrity and makes it easier to enforce rules such as unique emails, valid memberships, and reliable audit records.

## JWT Tokens
JWT tokens were chosen for authentication because they support stateless API access across multiple services. Compared with traditional server sessions, JWTs reduce database lookups on every request and work well in a distributed architecture. The trade-off is that token revocation is harder, and short-lived tokens plus session tracking may be needed for stronger control. In this project, JWTs benefit the user and team services by allowing secure login, protecting manager-only actions, and enabling consistent authorization checks across services.

## RabbitMQ
RabbitMQ was selected as the message broker because it is reliable for asynchronous event delivery and fits workflow-style communication between services. Compared with direct synchronous calls, RabbitMQ reduces tight coupling and lets services react to events such as team creation or asset updates. The trade-off is additional operational complexity and the need to manage retries, dead letters, and message formats. In this system, RabbitMQ supports integration between users, teams, assets, and the audit consumer while keeping core requests responsive.

## Redis
Redis was chosen for caching because it offers very fast in-memory access and is ideal for frequently read data. Compared with storing every lookup in PostgreSQL, Redis reduces repeated queries and improves response times. The trade-off is that cached data can become stale, so invalidation rules must be designed carefully. In this project, Redis improves performance for team member lists, asset metadata, and access control data, which are read often and benefit from quick retrieval.

## Docker and Docker Compose
Docker and Docker Compose were chosen to make the system reproducible and easy to run on any machine. Compared with manual environment setup, containers remove dependency issues and allow the microservices, PostgreSQL, Redis, and RabbitMQ to start together with one command. The trade-off is added build and orchestration complexity, especially when debugging container networking or volumes. For this project, Docker supports a realistic capstone deployment, simplifies setup for reviewers, and keeps the entire stack consistent across development and demonstration environments. It also makes the project easier to hand over, reproduce, and evaluate during submission.
That consistency was essential for testing all services together.