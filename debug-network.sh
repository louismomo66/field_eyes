#!/bin/bash

echo "=== Network Debugging Script ==="
echo ""

echo "Current container hostname: $(hostname)"
echo "Current container IP: $(hostname -I)"
echo ""

echo "=== DNS Configuration ==="
cat /etc/resolv.conf
echo ""

echo "=== Network Interfaces ==="
ip a
echo ""

echo "=== Testing connection to fieldeyes-db ==="
ping -c 3 fieldeyes-db || echo "Cannot ping fieldeyes-db"
nslookup fieldeyes-db || echo "Cannot resolve fieldeyes-db"
echo "Attempting direct connection to PostgreSQL..."
nc -zv fieldeyes-db 5432 || echo "Cannot connect to PostgreSQL"
echo ""

echo "=== Environment Variables ==="
echo "DATABASE_URL: $DATABASE_URL"
echo ""

echo "=== Testing fallback environment variables ==="
if [ -n "$DB_HOST" ]; then
  echo "DB_HOST: $DB_HOST"
  ping -c 1 $DB_HOST || echo "Cannot ping DB_HOST"
fi

echo "=== End of Network Debugging ===" 