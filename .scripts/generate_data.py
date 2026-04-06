#!/usr/bin/env python3
"""
Generate synthetic data for Trip Builder via API
NO hardcoded strings - all field names from class variables
"""

import requests
import random
import time
import subprocess
from datetime import datetime, timedelta
from typing import Dict, Optional, List

# ============================================================
# CONFIGURATION
# ============================================================

GATEWAY_URL = "http://localhost:8090"
API_PREFIX = "/api/v1"
FULL_URL = f"{GATEWAY_URL}{API_PREFIX}"

POSTGRES_CONTAINER = "trip-builder-backend-postgres"
POSTGRES_USER = "postgres"
POSTGRES_DB = "trip-builder"
POSTGRES_SCHEMA = "tpb_auth"

REQUEST_DELAY = 0.8

COUNT_USERS = 10
COUNT_ROUTES = 5
COUNT_EVENTS = 7
MAX_LIKES_PER_ROUTE = 3
MAX_PARTICIPANTS_PER_EVENT = 1
MIN_TASKS_PER_EVENT = 1
MAX_TASKS_PER_EVENT = 1
MIN_MESSAGES_PER_EVENT = 1
MAX_MESSAGES_PER_EVENT = 1

FIRST_NAMES = ["Alex", "Dmitry", "Maxim", "Vladimir", "Sergey", "Andrey", "Ekaterina", "Anna", "Maria", "Olga"]
LAST_NAMES = ["Ivanov", "Petrov", "Sidorov", "Smirnov", "Kuznetsov", "Popov", "Vasiliev", "Sokolov"]
EMAIL_DOMAIN = "test.com"
PASSWORD = "password123"

TAGS = ["history", "architecture", "russia", "beach", "mountain", "urban", "adventure", "family"]

class TaskTemplate:
    def __init__(self, title: str, description: str, priority: str, trigger_days: int):
        self.title = title
        self.description = description
        self.priority = priority
        self.trigger_days = trigger_days

TASK_TEMPLATES = [
    TaskTemplate("Book hotel", "Find and book hotel", "high", -14),
    TaskTemplate("Buy tickets", "Buy flight tickets", "high", -21),
    TaskTemplate("Pack suitcase", "Prepare things", "medium", -2),
    TaskTemplate("Buy gift", "Buy souvenir", "medium", -5),
    TaskTemplate("Organize meeting", "Meet at entrance", "high", -1),
]

MESSAGES = ["Hello everyone!", "Great place!", "Already booked", "How are you?", "Have a great day!"]

# ============================================================
# CITY CLASS
# ============================================================

class City:
    def __init__(self, name: str, country: str, lat: float, lng: float):
        self.name = name
        self.country = country
        self.lat = lat
        self.lng = lng


CITIES = [
    City("Moscow", "Russia", 55.7558, 37.6173),
    City("Saint Petersburg", "Russia", 59.9343, 30.3351),
    City("Sochi", "Russia", 43.5855, 39.7231),
    City("Kazan", "Russia", 55.7887, 49.1221),
    City("Paris", "France", 48.8566, 2.3522),
    City("London", "UK", 51.5074, -0.1278),
    City("Rome", "Italy", 41.9028, 12.4964),
    City("Barcelona", "Spain", 41.3851, 2.1734),
]

# ============================================================
# REQUEST CLASSES
# ============================================================

class RegisterRequest:
    EMAIL = "email"
    PASSWORD = "password"
    FIRST_NAME = "firstName"
    LAST_NAME = "lastName"

    def __init__(self):
        self._data = {}

    def set_email(self, value: str):
        self._data[self.EMAIL] = value
        return self

    def set_password(self, value: str):
        self._data[self.PASSWORD] = value
        return self

    def set_first_name(self, value: str):
        self._data[self.FIRST_NAME] = value
        return self

    def set_last_name(self, value: str):
        self._data[self.LAST_NAME] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class LoginRequest:
    EMAIL = "email"
    PASSWORD = "password"

    def __init__(self):
        self._data = {}

    def set_email(self, value: str):
        self._data[self.EMAIL] = value
        return self

    def set_password(self, value: str):
        self._data[self.PASSWORD] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class WaypointRequest:
    CITY = "city"
    DESCRIPTION = "description"
    LATITUDE = "latitude"
    LONGITUDE = "longitude"
    DURATION_DAYS = "duration_days"
    DURATION_HOURS = "duration_hours"
    ORDER_INDEX = "order_index"

    def __init__(self):
        self._data = {}

    def set_city(self, value: str):
        self._data[self.CITY] = value
        return self

    def set_description(self, value: str):
        self._data[self.DESCRIPTION] = value
        return self

    def set_latitude(self, value: float):
        self._data[self.LATITUDE] = value
        return self

    def set_longitude(self, value: float):
        self._data[self.LONGITUDE] = value
        return self

    def set_duration_days(self, value: int):
        self._data[self.DURATION_DAYS] = value
        return self

    def set_duration_hours(self, value: int):
        self._data[self.DURATION_HOURS] = value
        return self

    def set_order_index(self, value: int):
        self._data[self.ORDER_INDEX] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class CreateRouteRequest:
    TITLE = "title"
    DESCRIPTION = "description"
    COUNTRY = "country"
    IMAGE_URL = "image_url"
    IS_PUBLIC = "is_public"
    TAGS = "tags"
    WAYPOINTS = "waypoints"

    def __init__(self):
        self._data = {}

    def set_title(self, value: str):
        self._data[self.TITLE] = value
        return self

    def set_description(self, value: str):
        self._data[self.DESCRIPTION] = value
        return self

    def set_country(self, value: str):
        self._data[self.COUNTRY] = value
        return self

    def set_image_url(self, value: str):
        self._data[self.IMAGE_URL] = value
        return self

    def set_is_public(self, value: bool):
        self._data[self.IS_PUBLIC] = value
        return self

    def set_tags(self, value: List[str]):
        self._data[self.TAGS] = value
        return self

    def set_waypoints(self, value: List[Dict]):
        self._data[self.WAYPOINTS] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class CreateEventRequest:
    ROUTE_ID = "route_id"
    TITLE = "title"
    START_DATE = "start_date"
    IS_GROUP = "is_group"
    INVITED_EMAILS = "invited_emails"

    def __init__(self):
        self._data = {}

    def set_route_id(self, value: str):
        self._data[self.ROUTE_ID] = value
        return self

    def set_title(self, value: str):
        self._data[self.TITLE] = value
        return self

    def set_start_date(self, value: str):
        self._data[self.START_DATE] = value
        return self

    def set_is_group(self, value: bool):
        self._data[self.IS_GROUP] = value
        return self

    def set_invited_emails(self, value: List[str]):
        self._data[self.INVITED_EMAILS] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class CreateTaskRequest:
    EVENT_ID = "event_id"
    TITLE = "title"
    DESCRIPTION = "description"
    ASSIGNED_TO = "assigned_to"
    PRIORITY = "priority"
    DUE_DATE = "due_date"

    def __init__(self):
        self._data = {}

    def set_event_id(self, value: str):
        self._data[self.EVENT_ID] = value
        return self

    def set_title(self, value: str):
        self._data[self.TITLE] = value
        return self

    def set_description(self, value: str):
        self._data[self.DESCRIPTION] = value
        return self

    def set_assigned_to(self, value: str):
        self._data[self.ASSIGNED_TO] = value
        return self

    def set_priority(self, value: str):
        self._data[self.PRIORITY] = value
        return self

    def set_due_date(self, value: str):
        self._data[self.DUE_DATE] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


class SendMessageRequest:
    TEXT = "text"

    def __init__(self):
        self._data = {}

    def set_text(self, value: str):
        self._data[self.TEXT] = value
        return self

    def to_dict(self) -> Dict:
        return self._data


# ============================================================
# RESPONSE CLASSES
# ============================================================

class RegisterResponse:
    ID = "id"

    @staticmethod
    def get_id(data: Dict) -> Optional[str]:
        return data.get(RegisterResponse.ID) if data else None


class LoginResponse:
    ACCESS_TOKEN = "access_token"

    @staticmethod
    def get_access_token(data: Dict) -> Optional[str]:
        return data.get(LoginResponse.ACCESS_TOKEN) if data else None


class CreateRouteResponse:
    ID = "id"
    USER_ID = "user_id"
    TITLE = "title"

    @staticmethod
    def get_id(data: Dict) -> Optional[str]:
        return data.get(CreateRouteResponse.ID) if data else None

    @staticmethod
    def get_user_id(data: Dict) -> Optional[str]:
        return data.get(CreateRouteResponse.USER_ID) if data else None

    @staticmethod
    def get_title(data: Dict) -> Optional[str]:
        return data.get(CreateRouteResponse.TITLE) if data else None


class CreateEventResponse:
    ID = "id"
    OWNER_ID = "owner_id"
    IS_GROUP = "is_group"
    START_DATE = "start_date"

    @staticmethod
    def get_id(data: Dict) -> Optional[str]:
        return data.get(CreateEventResponse.ID) if data else None

    @staticmethod
    def get_owner_id(data: Dict) -> Optional[str]:
        return data.get(CreateEventResponse.OWNER_ID) if data else None

    @staticmethod
    def get_is_group(data: Dict) -> Optional[bool]:
        return data.get(CreateEventResponse.IS_GROUP) if data else None

    @staticmethod
    def get_start_date(data: Dict) -> Optional[str]:
        return data.get(CreateEventResponse.START_DATE) if data else None


class ParticipantsResponse:
    USER_ID = "user_id"

    @staticmethod
    def get_user_ids(data: List) -> List[str]:
        if not data:
            return []
        result = []
        for item in data:
            user_id = item.get(ParticipantsResponse.USER_ID)
            if user_id:
                result.append(user_id)
        return result


# ============================================================
# INTERNAL STORAGE CLASSES
# ============================================================

class User:
    def __init__(self, user_id: str, email: str):
        self.id = user_id
        self.email = email


class Route:
    def __init__(self, route_id: str, user_id: str, title: str):
        self.id = route_id
        self.user_id = user_id
        self.title = title


class Event:
    def __init__(self, event_id: str, owner_id: str, is_group: bool, start_date: str):
        self.id = event_id
        self.owner_id = owner_id
        self.is_group = is_group
        self.start_date = start_date


# ============================================================
# STORAGE
# ============================================================

users: List[User] = []
tokens: Dict[str, str] = {}
routes: List[Route] = []
events: List[Event] = []

# ============================================================
# UTILITIES
# ============================================================

def log_info(msg: str):
    print(f"[INFO] {msg}")

def log_success(msg: str):
    print(f"[OK] {msg}")

def log_error(msg: str):
    print(f"[ERR] {msg}")

def log_step(msg: str):
    print(f"\n=== {msg} ===")


def api_request(method: str, path: str, body: Dict = None, token: str = None) -> Optional[Dict]:
    url = f"{FULL_URL}{path}"
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"

    for retry in range(3):
        try:
            if method == "GET":
                resp = requests.get(url, headers=headers, timeout=30)
            elif method == "POST":
                resp = requests.post(url, headers=headers, json=body, timeout=30)
            else:
                return None

            if resp.status_code == 429:
                wait = (retry + 1) * 2
                log_info(f"Rate limited, retry {retry+1}/3 after {wait}s")
                time.sleep(wait)
                continue

            if resp.status_code >= 400:
                log_error(f"HTTP {resp.status_code} on {method} {path}")
                return None

            return resp.json() if resp.text else None

        except Exception as e:
            log_error(f"Request failed: {e}")
            if retry == 2:
                return None
            time.sleep(1)

    return None


def exec_sql(sql: str) -> None:
    cmd = f'docker exec {POSTGRES_CONTAINER} psql -U {POSTGRES_USER} -d {POSTGRES_DB} -c "{sql}"'
    subprocess.run(cmd, shell=True, capture_output=True)


def random_future_date(max_days: int = 60) -> str:
    date = datetime.now() + timedelta(days=random.randint(1, max_days))
    return date.strftime("%Y-%m-%dT00:00:00Z")


def random_waypoints(count: int) -> List[Dict]:
    waypoints = []
    used_city_names = set()

    for idx in range(count):
        city = random.choice(CITIES)
        while city.name in used_city_names:
            city = random.choice(CITIES)
        used_city_names.add(city.name)

        days = random.randint(0, 2)
        hours = random.randint(0, 8)
        if days == 0 and hours == 0:
            hours = 4

        req = WaypointRequest()
        req.set_city(city.name)
        req.set_description(f"Visit {city.name}")
        req.set_latitude(city.lat)
        req.set_longitude(city.lng)
        req.set_duration_days(days)
        req.set_duration_hours(hours)
        req.set_order_index(idx)

        waypoints.append(req.to_dict())

    return waypoints


# ============================================================
# 1. REGISTER USERS
# ============================================================
log_step(f"Registering {COUNT_USERS} users")

for i in range(1, COUNT_USERS + 1):
    first = random.choice(FIRST_NAMES)
    last = random.choice(LAST_NAMES)
    email = f"{first.lower()}.{last.lower()}{i}@{EMAIL_DOMAIN}"

    req = RegisterRequest()
    req.set_email(email)
    req.set_password(PASSWORD)
    req.set_first_name(first)
    req.set_last_name(last)

    resp = api_request("POST", "/auth/register", req.to_dict())
    user_id = RegisterResponse.get_id(resp)

    if user_id:
        users.append(User(user_id, email))
        if i % 10 == 0:
            log_info(f"Registered {i} users")

    time.sleep(REQUEST_DELAY)

log_success(f"Registered {len(users)} users")
if not users:
    exit(1)


# ============================================================
# 2. VERIFY USERS
# ============================================================
log_step("Verifying all users")

ids = ",".join([f"'{u.id}'" for u in users])
exec_sql(f"UPDATE {POSTGRES_SCHEMA}.users SET is_verified = true WHERE id IN ({ids});")
log_success("All users verified")


# ============================================================
# 3. GET TOKENS
# ============================================================
log_step("Getting tokens")

for user in users:
    req = LoginRequest()
    req.set_email(user.email)
    req.set_password(PASSWORD)

    resp = api_request("POST", "/auth/login", req.to_dict())
    token = LoginResponse.get_access_token(resp)

    if token:
        tokens[user.id] = token
    time.sleep(REQUEST_DELAY)

log_success(f"Obtained {len(tokens)} tokens")


# ============================================================
# 4. CREATE ROUTES
# ============================================================
log_step(f"Creating {COUNT_ROUTES} routes")

for i in range(1, COUNT_ROUTES + 1):
    user = random.choice(users)
    token = tokens[user.id]

    waypoint_count = random.randint(3, 6)
    waypoints = random_waypoints(waypoint_count)

    first_waypoint_city = None
    for wp in waypoints:
        first_waypoint_city = wp.get(WaypointRequest.CITY)
        break

    req = CreateRouteRequest()
    req.set_title(f"Route #{i}")
    req.set_description(f"Journey to {first_waypoint_city}")
    req.set_country(CITIES[0].country)
    req.set_image_url("")
    req.set_is_public(random.choice([True, False]))
    req.set_tags(random.sample(TAGS, random.randint(2, 4)))
    req.set_waypoints(waypoints)

    resp = api_request("POST", "/routes", req.to_dict(), token)
    route_id = CreateRouteResponse.get_id(resp)
    route_user_id = CreateRouteResponse.get_user_id(resp)
    route_title = CreateRouteResponse.get_title(resp)

    if route_id and route_user_id and route_title:
        routes.append(Route(route_id, route_user_id, route_title))
        if i % 20 == 0:
            log_info(f"Created {i} routes")

    time.sleep(REQUEST_DELAY)

log_success(f"Created {len(routes)} routes")


# ============================================================
# 5. ADD LIKES
# ============================================================
log_step("Adding likes")

total_likes = 0
for route in routes:
    count = random.randint(0, MAX_LIKES_PER_ROUTE)
    if count > 0:
        likers = random.sample(users, min(count, len(users)))
        for liker in likers:
            if liker.id != route.user_id:
                api_request("POST", f"/routes/{route.id}/like", token=tokens[liker.id])
                total_likes += 1
                time.sleep(REQUEST_DELAY)
            else:
                log_info(f"liker {liker.id} cant like")

log_success(f"Added {total_likes} likes")


# ============================================================
# 6. CREATE EVENTS
# ============================================================
log_step(f"Creating {COUNT_EVENTS} events")

for i in range(1, COUNT_EVENTS + 1):
    if not routes:
        break

    route = random.choice(routes)
    user = next((u for u in users if u.id == route.user_id), users[0])
    token = tokens[user.id]

    is_group = random.choice([True, False])

    invites = []
    if is_group:
        candidates = [u for u in users if u.id != user.id]
        if candidates:
            invites = [u.email for u in random.sample(candidates, min(MAX_PARTICIPANTS_PER_EVENT, len(candidates)))]

    req = CreateEventRequest()
    req.set_route_id(route.id)
    req.set_title(f"Trip along {route.title}")
    req.set_start_date(random_future_date(60))
    req.set_is_group(is_group)
    req.set_invited_emails(invites)

    resp = api_request("POST", "/events", req.to_dict(), token)
    event_id = CreateEventResponse.get_id(resp)
    event_owner_id = CreateEventResponse.get_owner_id(resp)
    event_is_group = CreateEventResponse.get_is_group(resp)
    event_start_date = CreateEventResponse.get_start_date(resp)

    if event_id and event_owner_id and event_is_group is not None and event_start_date:
        events.append(Event(event_id, event_owner_id, event_is_group, event_start_date))
        if i % 10 == 0:
            log_info(f"Created {i} events")

    time.sleep(REQUEST_DELAY)

log_success(f"Created {len(events)} events")


# ============================================================
# 7. JOIN EVENTS
# ============================================================
log_step("Joining events")

total_joins = 0
for event in events:
    if not event.is_group:
        continue

    candidates = [u for u in users if u.id != event.owner_id]
    if candidates:
        count = min(random.randint(1, MAX_PARTICIPANTS_PER_EVENT), len(candidates))
        for participant in random.sample(candidates, count):
            api_request("POST", f"/events/{event.id}/join", token=tokens[participant.id])
            total_joins += 1
            time.sleep(REQUEST_DELAY)

log_success(f"{total_joins} participants joined")


# ============================================================
# 8. CREATE TASKS
# ============================================================
log_step("Creating tasks")

total_tasks = 0



for event in events:
    owner = next((u for u in users if u.id == event.owner_id), users[0])
    token = tokens[owner.id]

    tasks_per_event = random.randint(MIN_TASKS_PER_EVENT, MAX_TASKS_PER_EVENT)
    templates = random.sample(TASK_TEMPLATES, min(tasks_per_event, len(TASK_TEMPLATES)))

    for template in templates:
        start_date = datetime.fromisoformat(event.start_date.replace("Z", "+00:00"))
        due_date = start_date + timedelta(days=template.trigger_days)

        req = CreateTaskRequest()
        req.set_event_id(event.id)
        req.set_title(template.title)
        req.set_description(template.description)
        req.set_priority(template.priority)
        req.set_due_date(due_date.strftime("%Y-%m-%dT12:00:00Z"))

        resp = api_request("POST", "/tasks", req.to_dict(), token)
        if resp:
            total_tasks += 1
        time.sleep(REQUEST_DELAY)

log_success(f"Created {total_tasks} tasks")


# ============================================================
# 9. SEND MESSAGES
# ============================================================
log_step("Sending messages")

total_messages = 0
for event in events:
    resp = api_request("GET", f"/events/{event.id}/participants", token=tokens[event.owner_id])
    participant_ids = ParticipantsResponse.get_user_ids(resp)

    all_participants = [event.owner_id] + participant_ids

    if len(all_participants) < 2:
        continue

    messages_per_event = random.randint(MIN_MESSAGES_PER_EVENT, MAX_MESSAGES_PER_EVENT)

    for _ in range(messages_per_event):
        sender = random.choice(all_participants)
        token = tokens[sender]

        req = SendMessageRequest()
        req.set_text(random.choice(MESSAGES))

        api_request("POST", f"/chat/events/{event.id}/messages", req.to_dict(), token)
        total_messages += 1

        if total_messages % 20 == 0:
            log_info(f"Sent {total_messages} messages")

        time.sleep(REQUEST_DELAY)

log_success(f"Sent {total_messages} messages")


# ============================================================
# 10. SYNC SEARCH SERVICE (direct container access)
# ============================================================
log_step("Syncing Elasticsearch")

if users and tokens:
    SEARCH_PORT = 8088
    SEARCH_URL = f"http://localhost:{SEARCH_PORT}"

    first_user = users[0]
    token = tokens[first_user.id]

    # Sync indexes
    url = f"{SEARCH_URL}/api/v1/search/sync"
    headers = {
        "Content-Type": "application/json",
        "X-User-ID": first_user.id
    }

    try:
        resp = requests.post(url, headers=headers, timeout=30)
        if resp.status_code == 200:
            log_success("Elasticsearch sync completed")
        else:
            log_error(f"Sync failed with status {resp.status_code}: {resp.text}")
    except Exception as e:
        log_error(f"Sync request failed: {e}")
else:
    log_error("No users/tokens available for search sync")

time.sleep(5)


# ============================================================
# 11. TEST SEARCH QUERIES
# ============================================================
log_step("Testing search queries")

if users and tokens:
    first_user = users[0]
    token = tokens[first_user.id]

    headers = {
        "Content-Type": "application/json",
        "X-User-ID": first_user.id
    }

    public_headers = {"Content-Type": "application/json"}

    # 11.1 Search routes (public)
    log_info("11.1 Searching routes by keyword...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/routes?q=route&size=5",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Found {data.get('total', 0)} routes")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.2 Search routes with pagination
    log_info("11.2 Searching routes with pagination...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/routes?page=1&size=3",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Page {data.get('page', 1)} of {data.get('total_pages', 1)} (total: {data.get('total', 0)})")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.3 Search events (public)
    log_info("11.3 Searching events by keyword...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/events?q=trip&size=5",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Found {data.get('total', 0)} events")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.4 Search users (requires auth)
    log_info("11.4 Searching users by name...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/users?q=alex&size=5",
            headers=headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Found {data.get('total', 0)} users")
            if data.get('items'):
                first = data['items'][0]
                log_info(f"    Example: {first.get('first_name', '?')} {first.get('last_name', '?')} (id: {first.get('id', '?')[:8]}...)")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.5 Auto-suggest
    log_info("11.5 Auto-suggest by prefix...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/suggest?q=mo&size=5",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            suggestions = data.get('suggestions', [])
            log_success(f"  Got {len(suggestions)} suggestions")
            for s in suggestions[:3]:
                log_info(f"    - {s.get('text', '?')} (type: {s.get('type', '?')})")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.6 Filter routes by country
    log_info("11.6 Filtering routes by country...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/routes?country=Russia&size=5",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Found {data.get('total', 0)} routes in Russia")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

    time.sleep(1)

    # 11.7 Search with empty query (should return all)
    log_info("11.7 Search with empty query (all routes)...")
    try:
        resp = requests.get(
            f"{SEARCH_URL}/api/v1/search/routes?size=5",
            headers=public_headers,
            timeout=10
        )
        if resp.status_code == 200:
            data = resp.json()
            log_success(f"  Returned {len(data.get('items', []))} of {data.get('total', 0)} total routes")
        else:
            log_error(f"  Failed: {resp.status_code}")
    except Exception as e:
        log_error(f"  Error: {e}")

else:
    log_error("No users/tokens available for search tests")

log_success("Search queries completed")


# ============================================================
# FINAL STATISTICS
# ============================================================
log_step("FINAL STATISTICS")
print()
print("+---------------------------------------------------+")
print("|                Trip Builder Data                |")
print("+---------------------------------------------------+")
print(f"|  Users:      {len(users):10}                         |")
print(f"|  Routes:     {len(routes):10}                         |")
print(f"|  Likes:      {total_likes:10}                         |")
print(f"|  Events:     {len(events):10}                         |")
print(f"|  Tasks:      {total_tasks:10}                         |")
print(f"|  Messages:   {total_messages:10}                         |")
print("+---------------------------------------------------+")
print()

log_success("Data generation completed!")