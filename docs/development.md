# Development

See the [README](../README.md#development) for the basic dev loop (`air` +
`npm run dev`). This page covers the local database.

## Inspect the development database

```sh
cd frontend
npm run db:studio
```

[Drizzle Studio](https://orm.drizzle.team/docs/drizzle-kit-studio) opens at
<https://local.drizzle.studio> and connects to the database at
`VISTO_DATABASE_PATH` from the repository's `.env` file (`./data/visto.db` if
unset). It's a development/debugging tool only — it doesn't run in the Visto
server or Docker image. Use a local database path, not a container-only path
such as `/data/visto.db`.

## Reset a development database after baseline changes

The development schema uses one baseline migration and does not upgrade
databases created from an earlier development schema, including the previous
multi-migration history. Stop Visto, then either point `VISTO_DATABASE_PATH`
at a new, empty SQLite file, or back up and remove your old development
database. Starting Visto again creates the current schema. This reset is only
needed for a database created before the current baseline — new databases
initialize correctly on first startup.

## Generate a database diagram

After Visto has created and migrated the local database:

```sh
cd frontend
npm run db:diagram
```

This opens the database at `VISTO_DATABASE_PATH` (or `./data/visto.db` if
unset) read-only and writes `docs/database-schema.mmd`. Run it again after
schema migrations to refresh the diagram.
