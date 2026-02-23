POST {{base_url}}/api/v1/auth/login
Content-Type: application/json
{
  "email": "{{auth_email}}",
  "password": "{{auth_password}}"
}
HTTP 200
[Captures]
access_token: jsonpath "$.access_token"
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.access_token" exists
jsonpath "$.token_type" == "Bearer"

GET {{base_url}}/api/v1/health
HTTP 200
[Asserts]
jsonpath "$.status" == "ok"

GET {{base_url}}/api/v1/clinics?limit=20
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"

POST {{base_url}}/api/v1/clinics
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "tax_id_number": "{{clinic_tax_id}}",
  "legal_name": "Odonto Company LTDA",
  "trade_name": "Odonto Prime",
  "email": "clinic@example.com",
  "phone": "+55 11 99999-9999",
  "bank_accounts": [
    {
      "bank_code": "001",
      "branch_number": "1234",
      "account_number": "998877"
    },
    {
      "bank_code": "341",
      "branch_number": "9999",
      "account_number": "111222"
    }
  ]
}
HTTP 201
[Captures]
clinic_id: jsonpath "$.id"
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.id" matches "^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-7[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$"
jsonpath "$.dentist_ids" count == 0

POST {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "tax_id_number": "{{dentist_tax_id}}",
  "legal_name": "Dr. Jane Doe",
  "email": "jane@example.com",
  "phone": "+55 21 98888-7777",
  "is_admin": true,
  "is_legal_representative": true
}
HTTP 201
[Captures]
dentist_id: jsonpath "$.id"

GET {{base_url}}/api/v1/clinics/{{clinic_id}}
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
jsonpath "$.id" == "{{clinic_id}}"
jsonpath "$.bank_accounts" count == 2
jsonpath "$.dentist_ids" count == 1

PATCH {{base_url}}/api/v1/clinics/{{clinic_id}}
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "trade_name": "Odonto Prime Centro",
  "bank_accounts": [
    {
      "bank_code": "104",
      "branch_number": "0001",
      "account_number": "123456-7"
    }
  ]
}
HTTP 200
[Asserts]
jsonpath "$.id" == "{{clinic_id}}"
jsonpath "$.dentist_ids" count == 1

GET {{base_url}}/api/v1/clinics/{{clinic_id}}
Authorization: Bearer {{access_token}}
HTTP 200
[Captures]
bank_account_id: jsonpath "$.bank_accounts[0].id"
[Asserts]
jsonpath "$.bank_accounts" count == 3

PATCH {{base_url}}/api/v1/clinics/{{clinic_id}}
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "bank_account_ids_to_remove": [
    "{{bank_account_id}}"
  ]
}
HTTP 200
[Asserts]
jsonpath "$.id" == "{{clinic_id}}"

GET {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists?limit=20
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
jsonpath "$" count == 1

PATCH {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists/{{dentist_id}}
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "is_admin": false
}
HTTP 200
[Asserts]
jsonpath "$.id" == "{{dentist_id}}"
jsonpath "$.is_admin" == false

GET {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists?limit=20
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
jsonpath "$" count == 1

PATCH {{base_url}}/api/v1/dentists/{{dentist_id}}
Content-Type: application/json
Authorization: Bearer {{access_token}}
{
  "legal_name": "Dr. Jane Doe Updated",
  "email": "jane.updated@example.com"
}
HTTP 200
[Asserts]
jsonpath "$.id" == "{{dentist_id}}"
jsonpath "$.legal_name" == "Dr. Jane Doe Updated"

DELETE {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists/{{dentist_id}}
Authorization: Bearer {{access_token}}
HTTP 409
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.type" == "https://capim.test/problems/conflict"

DELETE {{base_url}}/api/v1/dentists/{{dentist_id}}
Authorization: Bearer {{access_token}}
HTTP 204

GET {{base_url}}/api/v1/clinics/{{clinic_id}}/dentists?limit=20
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
jsonpath "$" count == 0

GET {{base_url}}/api/v1/clinics/{{clinic_id}}
Authorization: Bearer {{access_token}}
HTTP 200
[Asserts]
jsonpath "$.bank_accounts" count == 2

DELETE {{base_url}}/api/v1/clinics/{{clinic_id}}
Authorization: Bearer {{access_token}}
HTTP 204

GET {{base_url}}/api/v1/clinics/{{clinic_id}}
Authorization: Bearer {{access_token}}
HTTP 404
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.type" == "https://capim.test/problems/not-found"
