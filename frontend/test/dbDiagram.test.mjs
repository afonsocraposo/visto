import assert from "node:assert/strict";
import test from "node:test";
import Database from "better-sqlite3";
import { buildMermaidDiagram } from "../scripts/generate-db-diagram.mjs";

test("Given a SQLite schema, When generating its diagram, Then tables and relationships are described", () => {
  const database = new Database(":memory:");
  database.exec(`
    CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT NOT NULL);
    CREATE TABLE user_settings (
      user_id TEXT PRIMARY KEY REFERENCES users(id),
      timezone TEXT NOT NULL DEFAULT 'UTC'
    );
    CREATE TABLE sessions (
      id TEXT PRIMARY KEY,
      user_id TEXT NOT NULL REFERENCES users(id)
    );
  `);

  try {
    const diagram = buildMermaidDiagram(database);

    assert.match(diagram, /erDiagram/);
    assert.match(diagram, /text id PK/);
    assert.match(diagram, /text user_id PK, FK/);
    assert.match(diagram, /users \|\|--o\| user_settings : user_id/);
    assert.match(diagram, /users \|\|--o\{ sessions : user_id/);
  } finally {
    database.close();
  }
});

test("Given a database with no application tables, When generating its diagram, Then a clear error is returned", () => {
  const database = new Database(":memory:");

  try {
    assert.throws(() => buildMermaidDiagram(database), /No application tables found/);
  } finally {
    database.close();
  }
});
