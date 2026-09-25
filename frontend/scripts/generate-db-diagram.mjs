import Database from "better-sqlite3";
import { config } from "dotenv";
import { existsSync, mkdirSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const scriptDirectory = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(scriptDirectory, "../..");
const outputPath = resolve(projectRoot, "docs/database-schema.mmd");

export function buildMermaidDiagram(database) {
  const tables = database
    .prepare(
      "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name",
    )
    .all();

  if (tables.length === 0) {
    throw new Error("No application tables found in the SQLite database.");
  }

  const lines = [
    "%% Generated from the SQLite database. Run `npm run db:diagram` from frontend to refresh.",
    "erDiagram",
    "",
  ];
  const relations = [];

  for (const table of tables) {
    const columns = database.pragma(`table_info(${quoteIdentifier(table.name)})`);
    const foreignKeys = database.pragma(`foreign_key_list(${quoteIdentifier(table.name)})`);
    const foreignKeyColumns = new Set(foreignKeys.map((foreignKey) => foreignKey.from));

    lines.push(`  ${table.name} {`);
    for (const column of columns) {
      const type = (column.type || "unknown").trim().replace(/\s+/g, "_").toLowerCase();
      const markers = [];
      if (column.pk) markers.push("PK");
      if (foreignKeyColumns.has(column.name)) markers.push("FK");
      lines.push(`    ${type} ${column.name}${markers.length ? ` ${markers.join(", ")}` : ""}`);
    }
    lines.push("  }");
    lines.push("");

    const foreignKeyGroups = groupBy(foreignKeys, (foreignKey) => foreignKey.id);
    for (const group of foreignKeyGroups.values()) {
      const [foreignKey] = group;
      if (!tables.some((candidate) => candidate.name === foreignKey.table)) continue;
      const childColumns = group.map((item) => item.from);
      const cardinality = hasUniqueKey(database, table.name, columns, childColumns)
        ? "||--o|"
        : "||--o{";
      relations.push(
        `  ${foreignKey.table} ${cardinality} ${table.name} : ${childColumns.join("_")}`,
      );
    }
  }

  if (relations.length > 0) {
    lines.push(...relations.sort(), "");
  }

  return `${lines.join("\n").trimEnd()}\n`;
}

export function generateDatabaseDiagram() {
  config({ path: resolve(projectRoot, ".env") });

  const configuredPath = process.env.VISTO_DATABASE_PATH || "data/visto.db";
  const databasePath = isAbsolute(configuredPath)
    ? configuredPath
    : resolve(projectRoot, configuredPath);

  if (!existsSync(databasePath)) {
    throw new Error(
      `Database not found at ${databasePath}. Start Visto once to create it, or set VISTO_DATABASE_PATH in .env.`,
    );
  }

  const database = new Database(databasePath, { readonly: true, fileMustExist: true });
  try {
    const diagram = buildMermaidDiagram(database);
    mkdirSync(dirname(outputPath), { recursive: true });
    writeFileSync(outputPath, diagram);
  } finally {
    database.close();
  }

  console.log(`Wrote ${outputPath}`);
}

function hasUniqueKey(database, tableName, columns, foreignKeyColumns) {
  const primaryKeyColumns = columns
    .filter((column) => column.pk)
    .sort((left, right) => left.pk - right.pk)
    .map((column) => column.name);

  if (sameColumns(primaryKeyColumns, foreignKeyColumns)) return true;

  const indexes = database.pragma(`index_list(${quoteIdentifier(tableName)})`);
  return indexes
    .filter((index) => index.unique)
    .some((index) => {
      const indexedColumns = database
        .pragma(`index_info(${quoteIdentifier(index.name)})`)
        .map((column) => column.name);
      return sameColumns(indexedColumns, foreignKeyColumns);
    });
}

function sameColumns(left, right) {
  return left.length === right.length && left.every((column) => right.includes(column));
}

function groupBy(items, keyFor) {
  return items.reduce((groups, item) => {
    const key = keyFor(item);
    const group = groups.get(key) || [];
    group.push(item);
    groups.set(key, group);
    return groups;
  }, new Map());
}

function quoteIdentifier(identifier) {
  return `"${identifier.replaceAll('"', '""')}"`;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  try {
    generateDatabaseDiagram();
  } catch (error) {
    console.error(`Could not generate the database diagram: ${error.message}`);
    process.exitCode = 1;
  }
}
