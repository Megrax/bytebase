<template>
  <div class="flex flex-col w-full h-full overflow-y-auto">
    <div
      class="pt-3 pl-1 w-full flex justify-start items-center border-b border-b-gray-300"
    >
      <span
        class="-mb-px px-3 leading-9 rounded-t-md text-sm text-gray-500 border border-b-0 border-transparent cursor-pointer select-none"
        :class="
          state.selectedTab === 'table-list' &&
          'bg-white border-gray-300 text-gray-800'
        "
        @click="handleChangeTab('table-list')"
        >{{ $t("schema-editor.table-list") }}</span
      >
      <span
        class="-mb-px px-3 leading-9 rounded-t-md text-sm text-gray-500 border border-b-0 border-transparent cursor-pointer select-none"
        :class="
          state.selectedTab === 'raw-sql' &&
          'bg-white border-gray-300 text-gray-800'
        "
        @click="handleChangeTab('raw-sql')"
        >{{ $t("schema-editor.raw-sql") }}</span
      >
      <span
        v-if="isDev"
        class="-mb-px px-3 leading-9 rounded-t-md text-sm text-gray-500 border border-b-0 border-transparent cursor-pointer select-none"
        :class="
          state.selectedTab === 'schema-diagram' &&
          'bg-white border-gray-300 text-gray-800'
        "
        @click="handleChangeTab('schema-diagram')"
        >{{ $t("schema-diagram.self") }}</span
      >
    </div>

    <!-- List view -->
    <template v-if="state.selectedTab === 'table-list'">
      <div class="py-2 w-full flex justify-between items-center space-x-2">
        <button
          class="flex flex-row justify-center items-center border px-3 py-1 leading-6 rounded text-sm hover:bg-gray-100"
          @click="handleCreateNewTable"
        >
          <heroicons-outline:plus class="w-4 h-auto mr-1 text-gray-400" />
          {{ $t("schema-editor.actions.create-table") }}
        </button>
      </div>
      <!-- table list -->
      <div
        class="w-full h-auto grid auto-rows-auto border-y relative overflow-y-auto"
      >
        <!-- table header -->
        <div
          class="sticky top-0 z-10 grid grid-cols-[repeat(6,_minmax(0,_1fr))_32px] w-full border-b text-sm leading-6 select-none bg-gray-50 text-gray-400"
        >
          <span
            v-for="header in tableHeaderList"
            :key="header.key"
            class="table-header-item-container"
            >{{ header.label }}</span
          >
          <span></span>
        </div>
        <!-- table body -->
        <div class="w-full">
          <div
            v-for="(table, index) in tableList"
            :key="`${index}-${table.name}`"
            class="grid grid-cols-[repeat(6,_minmax(0,_1fr))_32px] text-sm even:bg-gray-50"
            :class="
              isDroppedTable(table) && 'text-red-700 !bg-red-50 opacity-70'
            "
          >
            <div class="table-body-item-container">
              <NEllipsis
                class="w-full cursor-pointer leading-6 my-2 hover:text-accent"
              >
                <span @click="handleTableItemClick(table)">
                  {{ table.name }}
                </span>
              </NEllipsis>
            </div>
            <div class="table-body-item-container">
              {{ table.rowCount }}
            </div>
            <div class="table-body-item-container">
              {{ bytesToString(table.dataSize) }}
            </div>
            <div class="table-body-item-container">
              {{ table.engine }}
            </div>
            <div class="table-body-item-container">
              {{ table.collation }}
            </div>
            <div class="table-body-item-container">
              {{ table.comment }}
            </div>
            <div class="w-full flex justify-start items-center">
              <n-tooltip v-if="!isDroppedTable(table)" trigger="hover">
                <template #trigger>
                  <heroicons:trash
                    class="w-[14px] h-auto text-gray-500 cursor-pointer hover:opacity-80"
                    @click="handleDropTable(table)"
                  />
                </template>
                <span>{{ $t("schema-editor.actions.drop-table") }}</span>
              </n-tooltip>
              <n-tooltip v-else trigger="hover">
                <template #trigger>
                  <heroicons:arrow-uturn-left
                    class="w-[14px] h-auto text-gray-500 cursor-pointer hover:opacity-80"
                    @click="handleRestoreTable(table)"
                  />
                </template>
                <span>{{ $t("schema-editor.actions.restore") }}</span>
              </n-tooltip>
            </div>
          </div>
        </div>
      </div>
    </template>
    <div
      v-else-if="state.selectedTab === 'raw-sql'"
      class="w-full h-full overflow-y-auto"
    >
      <div
        v-if="state.isFetchingDDL"
        class="w-full h-full min-h-[64px] flex justify-center items-center"
      >
        <BBSpin />
      </div>
      <template v-else>
        <HighlightCodeBlock
          v-if="state.statement !== ''"
          class="text-sm px-3 py-2 whitespace-pre-wrap break-all"
          language="sql"
          :code="state.statement"
        ></HighlightCodeBlock>
        <div v-else class="flex px-3 py-2 italic text-sm text-gray-600">
          {{ $t("schema-editor.nothing-changed") }}
        </div>
      </template>
    </div>
    <template v-else-if="state.selectedTab === 'schema-diagram'">
      <SchemaDiagram
        :key="currentTab.databaseId"
        :database="database"
        :table-list="dbSchemaStore.getTableListByDatabaseId(database.id)"
      />
    </template>
  </div>

  <TableNameModal
    v-if="state.tableNameModalContext !== undefined"
    :database-id="state.tableNameModalContext.databaseId"
    :schema-name="state.tableNameModalContext.schemaName"
    :table-name="state.tableNameModalContext.tableName"
    @close="state.tableNameModalContext = undefined"
  />
</template>

<script lang="ts" setup>
import { NEllipsis } from "naive-ui";
import { computed, reactive, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  generateUniqueTabId,
  useDBSchemaStore,
  useNotificationStore,
  useSchemaEditorStore,
} from "@/store";
import {
  DatabaseId,
  DatabaseTabContext,
  DatabaseSchema,
  SchemaEditorTabType,
  DatabaseEdit,
} from "@/types";
import { Table } from "@/types/schemaEditor/atomType";
import { bytesToString } from "@/utils";
import { diffSchema } from "@/utils/schemaEditor/diffSchema";
import HighlightCodeBlock from "@/components/HighlightCodeBlock";
import TableNameModal from "../Modals/TableNameModal.vue";
import SchemaDiagram from "@/components/SchemaDiagram";

type TabType = "table-list" | "schema-diagram" | "raw-sql";

interface LocalState {
  selectedTab: TabType;
  selectedSchema: string;
  isFetchingDDL: boolean;
  statement: string;
  tableNameModalContext?: {
    databaseId: DatabaseId;
    schemaName: string;
    tableName: string | undefined;
  };
}

const { t } = useI18n();
const editorStore = useSchemaEditorStore();
const dbSchemaStore = useDBSchemaStore();
const notificationStore = useNotificationStore();
const state = reactive<LocalState>({
  selectedTab: "table-list",
  selectedSchema: "",
  isFetchingDDL: false,
  statement: "",
});
const currentTab = editorStore.currentTab as DatabaseTabContext;
const databaseSchema = editorStore.databaseSchemaById.get(
  currentTab.databaseId
) as DatabaseSchema;
const database = databaseSchema.database;
const schemaList = databaseSchema.schemaList;
const tableList = computed(() => {
  return (
    schemaList.find((schema) => schema.name === state.selectedSchema)
      ?.tableList ?? []
  );
});

const tableHeaderList = computed(() => {
  return [
    {
      key: "name",
      label: t("schema-editor.database.name"),
    },
    {
      key: "raw-count",
      label: t("schema-editor.database.row-count"),
    },
    {
      key: "data-size",
      label: t("schema-editor.database.data-size"),
    },
    {
      key: "engine",
      label: t("schema-editor.database.engine"),
    },
    {
      key: "collation",
      label: t("schema-editor.database.collation"),
    },
    {
      key: "comment",
      label: t("schema-editor.database.comment"),
    },
  ];
});

watch(
  () => state.selectedTab,
  async () => {
    if (state.selectedTab === "raw-sql") {
      state.isFetchingDDL = true;
      const databaseEditList: DatabaseEdit[] = [];
      for (const schema of databaseSchema.schemaList) {
        const originSchema = databaseSchema.originSchemaList.find(
          (schema) => schema.name === schema.name
        );
        if (!originSchema) {
          continue;
        }
        const diffSchemaResult = diffSchema(database.id, originSchema, schema);
        if (
          diffSchemaResult.createTableList.length > 0 ||
          diffSchemaResult.alterTableList.length > 0 ||
          diffSchemaResult.renameTableList.length > 0 ||
          diffSchemaResult.dropTableList.length > 0
        ) {
          databaseEditList.push({
            databaseId: database.id,
            ...diffSchemaResult,
          });
        }
      }

      if (databaseEditList.length > 0) {
        const statementList: string[] = [];
        for (const databaseEdit of databaseEditList) {
          const databaseEditResult = await editorStore.postDatabaseEdit(
            databaseEdit
          );
          if (databaseEditResult.validateResultList.length > 0) {
            notificationStore.pushNotification({
              module: "bytebase",
              style: "CRITICAL",
              title: "Invalid request",
              description: databaseEditResult.validateResultList
                .map((result) => result.message)
                .join("\n"),
            });
            state.statement = "";
            return;
          }
          statementList.push(databaseEditResult.statement);
        }
        state.statement = statementList.join("\n");
      }
      state.isFetchingDDL = false;
    }
  }
);

const isDroppedTable = (table: Table) => {
  return table.status === "dropped";
};

const handleChangeTab = (tab: TabType) => {
  state.selectedTab = tab;
};

const handleCreateNewTable = () => {
  state.tableNameModalContext = {
    databaseId: database.id,
    schemaName: state.selectedSchema,
    tableName: undefined,
  };
};

const handleTableItemClick = (table: Table) => {
  editorStore.addTab({
    id: generateUniqueTabId(),
    type: SchemaEditorTabType.TabForTable,
    databaseId: database.id,
    schemaName: state.selectedSchema,
    tableId: table.id,
  });
};

const handleDropTable = (table: Table) => {
  editorStore.dropTable(database.id, state.selectedSchema, table);
};

const handleRestoreTable = (table: Table) => {
  editorStore.restoreTable(table);
};
</script>

<style scoped>
.table-header-item-container {
  @apply py-2 px-3;
}
.table-body-item-container {
  @apply w-full h-10 box-border p-px pl-3 pr-4 relative truncate leading-10;
}
</style>
