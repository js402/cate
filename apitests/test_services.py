import requests
from helpers import assert_status_code

def test_system_services(base_url):
    """Test that system services are available and correct."""
    response = requests.get(f"{base_url}/system/services")
    assert_status_code(response, 200)
    services = response.json()
    assert isinstance(services, list)
    # Check for expected service names
    expected_services = {
        "accessservice", "userservice", "chatservice",
        "modelservice", "downloadservice", "backendservice"
    }
    for service in expected_services:
        assert service in services, f"Missing service: {service}"
