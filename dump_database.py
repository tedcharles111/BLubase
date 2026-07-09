import requests

ANON_KEY = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE3ODA5NTQ4NjMsInJlZiI6Im9NVnN2MiIsInJvbGUiOiJhbm9uIn0.7E-uWH50Og-eAqsYNRBPQhXhOn4ovmhftlee2Es_0QU"
BASE = "https://blubase.onrender.com/sql/sql"
HEADERS = {"Authorization": f"Bearer {ANON_KEY}", "x-project-ref": "oMVsv2"}

# 1. Read the static schema (seed_schema.sql)
with open("seed_schema.sql", "r") as f:
    schema = f.read()

# 2. Get table list
r = requests.get(BASE, headers=HEADERS, params={"query": "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name"})
tables = [t['table_name'] for t in r.json()]

# 3. Dump data
inserts = []
for table in tables:
    if table in ["themultiverse_view"]:
        continue
    r = requests.get(BASE, headers=HEADERS, params={"query": f'SELECT * FROM "{table}"'})
    rows = r.json()
    if not rows:
        continue
    cols = list(rows[0].keys())
    for row in rows:
        values = []
        for c in cols:
            val = row[c]
            if val is None:
                values.append("NULL")
            elif isinstance(val, str):
                safe = val.replace("'", "''")
                values.append(f"'{safe}'")
            elif isinstance(val, bool):
                values.append(str(val).lower())
            else:
                values.append(str(val))
        values_str = ", ".join(values)
        inserts.append(f'INSERT INTO "{table}" ({", ".join(cols)}) VALUES ({values_str}) ON CONFLICT DO NOTHING;')

# 4. Write final seed.sql
with open("seed.sql", "w") as f:
    f.write(schema + "\n" + "\n".join(inserts))

print(f"Backup completed: {len(inserts)} rows written.")
