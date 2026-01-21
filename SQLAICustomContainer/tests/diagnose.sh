#!/bin/bash

# Diagnostic script to identify test issues
# Run this to see what's failing

echo "=== Docker Diagnostic ==="
echo ""

echo "1. Docker version:"
docker --version 2>&1
echo ""

echo "2. Docker daemon status:"
docker info 2>&1 | head -n 5
echo ""

echo "3. Docker permissions (can run docker ps?):"
docker ps 2>&1 | head -n 3
echo ""

echo "4. sqlcmd installed:"
which sqlcmd 2>&1
echo ""

echo "5. sqlcmd version:"
sqlcmd -? 2>&1 | head -n 1
echo ""

echo "6. SA_PASSWORD set:"
if [ -n "$SA_PASSWORD" ]; then
    echo "✅ SA_PASSWORD is set"
    # Validate password complexity
    if [[ ${#SA_PASSWORD} -ge 8 ]] && \
       [[ "$SA_PASSWORD" =~ [A-Z] ]] && \
       [[ "$SA_PASSWORD" =~ [a-z] ]] && \
       [[ "$SA_PASSWORD" =~ [0-9] ]] && \
       [[ "$SA_PASSWORD" =~ [^a-zA-Z0-9] ]]; then
        echo "✅ Password meets SQL Server complexity requirements"
    else
        echo "❌ Password does not meet SQL Server requirements"
        echo "   Requirements: 8+ chars, uppercase, lowercase, digit, special character"
    fi
else
    echo "❌ SA_PASSWORD is NOT set"
    echo "   Run: export SA_PASSWORD='YourComplexPass@123'"
fi
echo ""

echo "7. Disk space:"
df -h . 2>&1 | head -n 2
echo ""

echo "8. Build script exists:"
if [ -f "../build-and-run.sh" ]; then
    echo "✅ build-and-run.sh found"
    ls -la ../build-and-run.sh
else
    echo "❌ build-and-run.sh NOT found"
fi
echo ""

echo "9. Test script permissions:"
ls -la *.sh
echo ""

echo "=== End Diagnostic ==="
