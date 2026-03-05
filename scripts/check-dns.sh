#!/bin/bash

# DNS Check Script for relay.name
# Usage: ./check-dns.sh [domain]

DOMAIN="${1:-relay.name}"
SERVER_IP="${2:-}"

echo "Checking DNS configuration for $DOMAIN"
echo "========================================"
echo ""

# Check root domain
echo "1. Root domain ($DOMAIN):"
ROOT_IP=$(dig +short $DOMAIN @8.8.8.8 | head -1)
if [ -z "$ROOT_IP" ]; then
    echo "   ❌ Not configured or not propagated"
else
    echo "   ✅ Resolves to: $ROOT_IP"
    if [ -n "$SERVER_IP" ] && [ "$ROOT_IP" != "$SERVER_IP" ]; then
        echo "   ⚠️  Warning: IP doesn't match expected server IP ($SERVER_IP)"
    fi
fi
echo ""

# Check wildcard
echo "2. Wildcard (*.$DOMAIN):"
WILDCARD_IP=$(dig +short "*.${DOMAIN}" @8.8.8.8 | head -1)
if [ -z "$WILDCARD_IP" ]; then
    echo "   ❌ Not configured or not propagated"
else
    echo "   ✅ Resolves to: $WILDCARD_IP"
    if [ -n "$SERVER_IP" ] && [ "$WILDCARD_IP" != "$SERVER_IP" ]; then
        echo "   ⚠️  Warning: IP doesn't match expected server IP ($SERVER_IP)"
    fi
fi
echo ""

# Check nameservers
echo "3. Nameservers:"
NAMESERVERS=$(dig +short NS $DOMAIN @8.8.8.8)
if [ -z "$NAMESERVERS" ]; then
    echo "   ❌ No nameservers found"
else
    echo "   ✅ Nameservers:"
    echo "$NAMESERVERS" | while read ns; do
        echo "      - $ns"
    done
fi
echo ""

# Test subdomain
echo "4. Test subdomain (test.$DOMAIN):"
TEST_IP=$(dig +short "test.${DOMAIN}" @8.8.8.8 | head -1)
if [ -z "$TEST_IP" ]; then
    echo "   ❌ Subdomain not resolving (wildcard may not be configured)"
else
    echo "   ✅ Resolves to: $TEST_IP"
fi
echo ""

# Check propagation
echo "5. DNS Propagation Check:"
echo "   Checking multiple DNS servers..."
for dns in "8.8.8.8" "1.1.1.1" "208.67.222.222"; do
    IP=$(dig +short $DOMAIN @$dns | head -1)
    if [ -n "$IP" ]; then
        echo "   ✅ $dns: $IP"
    else
        echo "   ❌ $dns: Not resolved"
    fi
done
echo ""

# Summary
echo "========================================"
if [ -n "$ROOT_IP" ] && [ -n "$WILDCARD_IP" ]; then
    echo "✅ DNS appears to be configured correctly!"
    echo ""
    echo "Next steps:"
    echo "1. Ensure your server is running: ./relay-server"
    echo "2. Test HTTP: curl http://test.$DOMAIN"
    echo "3. Create a DNS record via API"
else
    echo "❌ DNS configuration incomplete"
    echo ""
    echo "Please configure:"
    echo "1. Root A record: $DOMAIN → YOUR_SERVER_IP"
    echo "2. Wildcard A record: *.$DOMAIN → YOUR_SERVER_IP"
    echo ""
    echo "See DNS_SETUP.md for detailed instructions"
fi


