import { v1 as uuidv1 } from "uuid";
import {
  ColumnMetadata,
  SchemaMetadata,
  TableMetadata,
} from "../proto/store/database";

type AtomResourceStatus = "normal" | "created" | "dropped";

export interface Column {
  id: string;
  name: string;
  type: string;
  nullable: boolean;
  comment: string;
  default?: string;
  status: AtomResourceStatus;
}

export interface PrimaryKey {
  name: string;
  columnIdList: string[];
}

export interface Table {
  id: string;
  name: string;
  engine: string;
  collation: string;
  rowCount: number;
  dataSize: number;
  comment: string;
  columnList: Column[];
  // Including column id list.
  primaryKey: PrimaryKey;
  status: AtomResourceStatus;
}

export interface ForeignKey {
  // Should be an unique name.
  name: string;
  tableId: string;
  columnIdList: string[];
  referencedSchema: string;
  referencedTableId: string;
  referencedColumnIdList: string[];
}

export interface Schema {
  // It should be an empty string for MySQL/TiDB.
  name: string;
  tableList: Table[];
  foreignKeyList: ForeignKey[];
  status: AtomResourceStatus;
}

export const convertColumnMetadataToColumn = (
  columnMetadata: ColumnMetadata
): Column => {
  return {
    id: uuidv1(),
    name: columnMetadata.name,
    type: columnMetadata.type,
    nullable: columnMetadata.nullable,
    comment: columnMetadata.comment,
    default: columnMetadata.default,
    status: "normal",
  };
};

export const convertTableMetadataToTable = (
  tableMetadata: TableMetadata
): Table => {
  const table: Table = {
    id: uuidv1(),
    name: tableMetadata.name,
    engine: tableMetadata.engine,
    collation: tableMetadata.collation,
    rowCount: tableMetadata.rowCount,
    dataSize: tableMetadata.dataSize,
    comment: tableMetadata.comment,
    columnList: tableMetadata.columns.map((column) =>
      convertColumnMetadataToColumn(column)
    ),
    primaryKey: {
      name: "",
      columnIdList: [],
    },
    status: "normal",
  };

  for (const indexMetadata of tableMetadata.indexes) {
    if (indexMetadata.primary === true) {
      table.primaryKey.name = indexMetadata.name;
      for (const columnName of indexMetadata.expressions) {
        const column = table.columnList.find(
          (column) => column.name === columnName
        );
        if (column) {
          table.primaryKey.columnIdList.push(column.id);
        }
      }
      break;
    }
  }

  return table;
};

export const convertSchemaMetadataToSchema = (
  schemaMetadata: SchemaMetadata
): Schema => {
  const tableList: Table[] = [];
  const foreignKeyList: ForeignKey[] = [];

  for (const tableMetadata of schemaMetadata.tables) {
    const table = convertTableMetadataToTable(tableMetadata);
    tableList.push(table);
  }

  for (const tableMetadata of schemaMetadata.tables) {
    const table = tableList.find((table) => table.name === tableMetadata.name);
    if (!table) {
      continue;
    }

    for (const foreignKeyMetadata of tableMetadata.foreignKeys) {
      // TODO(steven): remove this after backend return unique fk.
      if (
        foreignKeyList.map((fk) => fk.name).includes(foreignKeyMetadata.name)
      ) {
        continue;
      }
      const referencedTable = tableList.find(
        (table) => table.name === foreignKeyMetadata.referencedTable
      );
      if (!referencedTable) {
        continue;
      }

      const fk: ForeignKey = {
        name: foreignKeyMetadata.name,
        tableId: table.id,
        columnIdList: [],
        referencedSchema: foreignKeyMetadata.referencedSchema,
        referencedTableId: referencedTable.id,
        referencedColumnIdList: [],
      };

      for (const columnName of foreignKeyMetadata.columns) {
        const column = table.columnList.find(
          (column) => column.name === columnName
        );
        if (column) {
          fk.columnIdList.push(column.id);
        }
      }
      for (const referencedColumnName of foreignKeyMetadata.referencedColumns) {
        const referencedColumn = referencedTable.columnList.find(
          (column) => column.name === referencedColumnName
        );
        if (referencedColumn) {
          fk.referencedColumnIdList.push(referencedColumn.id);
        }
      }

      foreignKeyList.push(fk);
    }
  }

  return {
    name: schemaMetadata.name,
    tableList: tableList,
    foreignKeyList: foreignKeyList,
    status: "normal",
  };
};
