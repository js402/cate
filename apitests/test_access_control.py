import requests
from helpers import assert_status_code

def test_assign_manage_permission(base_url, generate_email, register_user, admin_session):
    """
    Test that an admin can assign manage permission on the 'server' resource
    to a randomly registered user.
    """
    # Register a random user
    random_email = generate_email("access")
    password = "testpassword"
    register_user(random_email, "Test Access User", password)

    # Using admin session to assign permissions to the user for resource "server"
    payload = {
        "identity": random_email,
        "resource": "server",
        "permission": 3  # PermissionManage (0: none, 1: view, 2: edit, 3: manage)
    }
    headers = admin_session
    response = requests.post(f"{base_url}/access-control", json=payload, headers=headers)
    assert_status_code(response, 201)

    # Optionally, verify the permission was set by listing access entries for the user
    list_response = requests.get(f"{base_url}/access-control?identity={random_email}", headers=headers)
    assert_status_code(list_response, 200)
    entries = list_response.json()
    print(entries)
    # Check that there's an entry for the 'server' resource with manage permission.
    found = any(entry.get("resource") == "server" and entry.get("permission") == 3 for entry in entries)
    assert found, "Access entry for managing 'server' was not found for the user."
