import pytest
import uuid
import requests
import logging

BASE_URL = "http://localhost:8081"

# Configure the root logger
logging.basicConfig(level=logging.ERROR,
                    format='%(asctime)s - %(levelname)s - %(name)s - %(message)s')

# Create a logger object for your test module
logger = logging.getLogger(__name__)

@pytest.fixture(scope="session")
def base_url():
    logger.debug("Providing base URL: %s", BASE_URL)
    return BASE_URL

@pytest.fixture
def generate_email():
    """Generate a unique email address."""
    def _generate(prefix="user"):
        email = f"{prefix}_{uuid.uuid4().hex[:8]}@example.com"
        logger.debug("Generated email: %s", email)
        return email
    return _generate

@pytest.fixture(scope="session")
def admin_email():
    """Default admin email."""
    return "admin@admin.com"

@pytest.fixture(scope="session")
def admin_password():
    """Default admin password."""
    return "admin123"

@pytest.fixture
def register_user(base_url):
    """
    Fixture that registers a user and returns their token.
    Usage:
        token = register_user(email, friendly_name, password)
    """
    def _register(email, friendly_name, password):
        user_data = {
            "email": email,
            "friendlyName": friendly_name,
            "password": password
        }
        logger.info("Registering user: %s", email)
        try:
            response = requests.post(f"{base_url}/register", json=user_data)
            response.raise_for_status()
            logger.debug("User registration response: %s", response.json())
        except requests.RequestException as e:
            logger.exception("User registration failed for %s: %s", email, e)
            pytest.fail(f"Registration failed: {e}")
        assert response.status_code == 201, f"Registration failed: {response.text}"
        token = response.json().get("token", "")
        logger.info("User registered successfully, token obtained.")
        return token
    return _register

@pytest.fixture
def auth_headers(register_user, base_url, generate_email):
    """
    Fixture that registers a user and returns the authorization headers.
    """
    email = generate_email("auth")
    token = register_user(email, "Test User", "password123")
    logger.debug("Auth headers created for %s", email)
    return {"Authorization": f"Bearer {token}"}


@pytest.fixture(scope="session")
def admin_session(base_url, admin_email, admin_password):
    """
    Registers an admin user and returns authentication headers with session scope.
    """
    admin_data = {
        "email": admin_email,
        "friendlyName": "Admin User",
        "password": admin_password
    }
    logger.info("Registering admin user: %s", admin_email)
    try:
        response = requests.post(f"{base_url}/register", json=admin_data)
        response.raise_for_status()
        logger.debug("Admin registration response: %s", response.json())
    except requests.RequestException as e:
        logger.exception("Admin registration failed: %s", e)
        pytest.fail(f"Admin registration failed: {e}")
    assert response.status_code == 201, f"Admin registration failed: {response.text}"
    token = response.json().get("token", "")
    logger.info("Admin registered successfully, token obtained.")
    return {"Authorization": f"Bearer {token}"}
