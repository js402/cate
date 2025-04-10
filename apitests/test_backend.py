import requests
from helpers import assert_status_code

def test_create_backend(base_url, admin_session):
    """Test that an admin user can create a backend."""
    headers = admin_session
    payload = {
        "name": "Test backend",
        "baseUrl": "http://backend.example.com",
        "type": "Ollama",
        "model": "testmodel",
    }
    response = requests.post(f"{base_url}/backends", json=payload, headers=headers)
    assert_status_code(response, 201)

def test_list_backends(base_url, admin_session):
    """Test that an admin user can list backends."""
    headers = admin_session
    response = requests.get(f"{base_url}/backends", headers=headers)
    assert_status_code(response, 200)
    backends = response.json()
    assert isinstance(backends, list)

def test_list_backends_unauthorized(base_url, generate_email, register_user):
    """Test that a random user gets a 401 when listing backends."""
    email = generate_email("unauthorized")
    password = "unauthorizedpassword"
    token = register_user(email, "Unauthorized User", password)
    headers = {"Authorization": f"Bearer {token}"}
    response = requests.get(f"{base_url}/backends", headers=headers)
    assert_status_code(response, 401)
