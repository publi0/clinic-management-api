POST {{base_url}}/api/v1/auth/login
Content-Type: application/json
{
  "email": "wrong@example.com",
  "password": "invalid-password"
}
HTTP 401
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.type" == "https://capim.test/problems/unauthorized"
jsonpath "$.title" == "Unauthorized"
jsonpath "$.status" == 401

POST {{base_url}}/api/v1/auth/login
Content-Type: application/json
{
  "email": "{{auth_email}}",
  "password": "{{auth_password}}"
}
HTTP 200
[Captures]
access_token: jsonpath "$.access_token"

GET {{base_url}}/api/v1/clinics/invalid-id
Authorization: Bearer {{access_token}}
HTTP 400
[Asserts]
header "Content-Type" contains "application/problem+json"
jsonpath "$.type" == "https://capim.test/problems/invalid-parameter"
jsonpath "$.title" == "Invalid Parameter"
jsonpath "$.status" == 400
jsonpath "$.request_id" exists
