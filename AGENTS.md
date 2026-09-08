# MetaLab agent instructions

## PostgreSQL integration tests

- Always set `ML_TEST_DATABASE_URL` with a non-empty username and password, including for a temporary local cluster configured with `trust` authentication.
- Use the standard test identity and URL whenever possible: `postgres://metalab:metalab@127.0.0.1:<port>/metalab?sslmode=disable`.
- Initialize a temporary PostgreSQL cluster with the `metalab` superuser, or create that role before running the suite.
- `TestEnsureIdentityRejectsMLSystemIntegration` intentionally rejects an empty PostgreSQL password. A `PostgreSQL password is required` failure means the local test URL is invalid, not that application identity logic regressed.
- Before reporting an integration failure, verify that the local URL follows this rule and rerun the failing test once with the standard identity.
