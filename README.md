# CATE: Cognitive AI/Agent Transformation Engine

A modular platform for building context-aware agents, semantic search, and task automation — grounded in your data.

## 🚀 Project Vision

CATE aims to become a platform for semantic search and user-defined AI agents that operate within specific contexts.

The project's vision is focused on delivering these core features:

- **Document Ingestion**: Upload PDFs, text files, or URLs to build a knowledge base.
- **Semantic Search**: Search for relevant information within the declared knowledge base.
- **Contextual Chat Sessions**: Ask questions and get answers grounded in your documents.
- **Task Handling**: Create templates for repetitive tasks and execute them with user input.
- **Triggers**: Define conditions that trigger actions based on semantic matches.
- **Steps**: For complex requests, let the agent split the request into a chain of prompts.

## 🔧 What's Under the Hood

CATE combines several technologies to deliver its features:

- **Core Logic**: The main backend service and LLM Gateway, built in **Go**, provides the primary API and orchestration.
- **User Interface**: Dashboards and user interactions are handled by a **React** frontend.
- **Auxiliary Services**: Data processing tasks like document mining and indexing are handled by separate services written in **Python**.
- **Persistence**:
    - **PostgreSQL**: Used for core application state, metadata, and storing original uploaded documents.
    - **OpenSearch**: Manages vector embeddings for semantic search and stores chat history data.
    - **Valkey (Redis Fork)**: Used for caching and managing ephemeral runtime state.
- **Language Models**: Core AI capabilities rely on **Ollama** backends.
- **Security**:
    - Authentication is handled via **JWT tokens**.
    - A **Backend-for-Frontend (BFF)** pattern helps the UI securely manage token lifecycles.
    - User management and permissions utilize a **custom Access Control system**.
- **Inter-Service Communication**: **NATS** serves as the message bus for asynchronous coordination.
- **Deployment**:
    - The system is designed to run **containerized** (e.g., using Docker).
    - Users are expected to provide external dependencies like **PostgreSQL** and **Valkey**.
    - A `docker-compose.yml` file is provided for convenience, but operators typically deploy the CATE container image(s) directly and manage configuration externally.

## ⚙️ WIP - Work In Progress

This project is currently under heavy development. Please check back later for more detailed instructions. Demos, installation, and usage guides will be available within the next day.
