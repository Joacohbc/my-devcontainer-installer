##
## DATABASE CLIENT TOOLS (Redis, Postgres, Mongo 8.0)
##

# 1. Setup MongoDB 8.0 Repository for 'mongosh'
RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /etc/apt/keyrings/mongodb-server-8.0.gpg && \
    echo "deb [ arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/mongodb-server-8.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/8.0 multiverse" | tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# 2. Install all DB clients
RUN apt-get update && apt-get install -y \
    postgresql-client \
    redis-tools \
    mongodb-mongosh
