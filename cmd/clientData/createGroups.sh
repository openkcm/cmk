cd ../..
make build-tenant-cli
make provision-tenants-k3d
make tenant-cli ARGS="add-default-groups -i tenant1"
make tenant-cli ARGS="add-default-groups -i tenant2"

cd cmd/clientData
echo "Generating client headers for tenantAdmin..."
go run generate_client_headers.go -json tenantAdmin_clientData.json -out tenantAdmin_clientData_headers.txt

echo -e "\nCreating group for tenant1..."

# Extract headers from the generated file
CLIENT_DATA=$(grep "x-client-data:" tenantAdmin_clientData_headers.txt | cut -d' ' -f2-)
CLIENT_DATA_SIGNATURE=$(grep "x-client-data-signature:" tenantAdmin_clientData_headers.txt | cut -d' ' -f2-)

echo "Using generated headers from file"
echo "CLIENT_DATA: $CLIENT_DATA"
echo "CLIENT_DATA_SIGNATURE: $CLIENT_DATA_SIGNATURE"

response=$(curl --request POST \
  --url http://127.0.0.1:8080/cmk/v1/tenant1/groups \
  --header 'content-type: application/json' \
  --header "x-client-data: $CLIENT_DATA" \
  --header "x-client-data-signature: $CLIENT_DATA_SIGNATURE" \
  --data '{
  "name": "Tenant1-key-admin",
  "role": "KEY_ADMINISTRATOR",
  "description": "This group represents a Tenant Administrator"
}')

echo "Response from API:"
echo "$response"

go run generate_client_headers.go -json keyAdmin_clientData.json -out keyAdmin_clientData_headers.txt