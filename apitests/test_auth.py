import requests
from helpers import assert_status_code

def test_server_ok(base_url):
    """Test that the server root endpoint is accessible."""
    response = requests.get(f"{base_url}/")
    assert_status_code(response, 200)

def test_register(base_url, generate_email):
    """Test that a new user can register successfully."""
    email = generate_email("newuser")
    response = requests.post(f"{base_url}/register", json={
        "email": email,
        "friendlyName": "Jane Doe",
        "password": "newpassword"
    })
    assert_status_code(response, 201)
    token = response.json().get("token", "")
    assert token and isinstance(token, str)

def test_register_twice(base_url, generate_email):
    """Test that attempting to register the same email twice fails."""
    email = generate_email("duplicate")
    # First registration should succeed.
    response = requests.post(f"{base_url}/register", json={
        "email": email,
        "friendlyName": "John Doe",
        "password": "testpassword"
    })
    assert_status_code(response, 201)
    # Second registration should fail with 403 or 409.
    response = requests.post(f"{base_url}/register", json={
        "email": email,
        "friendlyName": "John Doe",
        "password": "testpassword"
    })
    assert response.status_code in (403, 409), f"Unexpected status: {response.status_code}"
