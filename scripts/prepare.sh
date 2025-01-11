#!/bin/bash

ENV_VARS=("DB_NAME" "DB_USER" "DB_PASSWD" "DB_HOST" "DB_PORT")

echo "Beginning setup..."

# Set up environment variables
export $(grep -v '^#' .env | xargs)

# Install Go dependencies
echo "Installing Go dependencies..."
go mod tidy

# Create the 'prices' table in the database
echo "Creating 'prices' table..."
PGPASSWORD=$DB_PASSWD psql -U $DB_USER -h $DB_HOST -p $DB_PORT -d $DB_NAME -c "
CREATE TABLE IF NOT EXISTS prices (
    id SERIAL PRIMARY KEY,
    manufacture_date DATE NOT NULL,
    product_name VARCHAR(255) NOT NULL,
    product_category VARCHAR(255) NOT NULL,
    product_price DECIMAL(10,2) NOT NULL
);"

echo "Setup complete!"