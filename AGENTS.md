# MetaLab agent instructions

## PostgreSQL integration tests

- Always set `ML_TEST_DATABASE_URL` with a non-empty username and password, including for a temporary local cluster configured with `trust` authentication.
- Use the standard test identity and URL whenever possible: `postgres://metalab:metalab@127.0.0.1:<port>/metalab?sslmode=disable`.
- Initialize a temporary PostgreSQL cluster with the `metalab` superuser, or create that role before running the suite.
- `TestEnsureIdentityRejectsMLSystemIntegration` intentionally rejects an empty PostgreSQL password. A `PostgreSQL password is required` failure means the local test URL is invalid, not that application identity logic regressed.
- Before reporting an integration failure, verify that the local URL follows this rule and rerun the failing test once with the standard identity.
- `ML_TEST_ADMIN_DATABASE_URL` follows the same rule: an empty password on a TCP connection is intentionally rejected (ML enforces "password required for TCP admin connections"), not a regression. Use a dedicated superuser role with a password, never the OS user via trust/peer auth.
- One-time local setup for a Homebrew/local PostgreSQL cluster already running on 127.0.0.1:5432:
  ```sql
  CREATE ROLE metalab LOGIN PASSWORD 'metalab';
  CREATE DATABASE metalab OWNER metalab;
  CREATE ROLE ml_test_admin LOGIN SUPERUSER PASSWORD 'ml_test_admin_pw';
  ```
  Then:
  ```sh
  export ML_TEST_DATABASE_URL="postgres://metalab:metalab@127.0.0.1:5432/metalab?sslmode=disable"
  export ML_TEST_ADMIN_DATABASE_URL="postgres://ml_test_admin:ml_test_admin_pw@127.0.0.1:5432/postgres?sslmode=disable"
  ```
  These roles are test-only fixtures (`ml_test_admin` is a superuser strictly for provisioning throwaway databases/roles in integration tests) — do not reuse them for anything else, and do not create them on a shared/production cluster.
