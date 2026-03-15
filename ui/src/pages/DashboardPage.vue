<template>
  <q-page padding>
    <div class="row q-col-gutter-md q-mb-lg">
      <div class="col-auto">
        <q-card class="stat-card">
          <q-card-section>
            <div class="text-caption text-grey">Active Tunnels</div>
            <div class="text-h4">{{ tunnels.length }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-auto">
        <q-card class="stat-card">
          <q-card-section>
            <div class="text-caption text-grey">Total Requests</div>
            <div class="text-h4">{{ totalRequests }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <q-table
      title="Active Tunnels"
      :rows="tunnels"
      :columns="columns"
      row-key="subdomain"
      flat
      bordered
      :pagination="{ rowsPerPage: 20 }"
      no-data-label="No active tunnels"
    >
      <template #body-cell-status="props">
        <q-td :props="props">
          <span class="tunnel-status" />
        </q-td>
      </template>

      <template #body-cell-subdomain="props">
        <q-td :props="props">
          <span class="mono text-weight-medium">{{ props.row.subdomain }}</span>
        </q-td>
      </template>

      <template #body-cell-url="props">
        <q-td :props="props">
          <a :href="tunnelUrl(props.row.subdomain)" target="_blank" class="mono">
            {{ tunnelUrl(props.row.subdomain) }}
          </a>
        </q-td>
      </template>

      <template #body-cell-uptime="props">
        <q-td :props="props">
          {{ formatUptime(props.row.createdAt) }}
        </q-td>
      </template>

      <template #body-cell-lastRequest="props">
        <q-td :props="props">
          {{ props.row.lastRequest ? formatTimestamp(props.row.lastRequest) : '-' }}
        </q-td>
      </template>
    </q-table>
  </q-page>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useDashboard } from 'src/composables/useDashboard';
import type { QTableColumn } from 'quasar';

const { tunnels } = useDashboard();

const totalRequests = computed(() =>
  tunnels.value.reduce((sum, t) => sum + t.requestCount, 0)
);

const columns: QTableColumn[] = [
  { name: 'status', label: '', field: 'subdomain', align: 'center', style: 'width: 30px' },
  { name: 'subdomain', label: 'Subdomain', field: 'subdomain', align: 'left', sortable: true },
  { name: 'url', label: 'URL', field: 'subdomain', align: 'left' },
  { name: 'apiKeyLabel', label: 'Client', field: 'apiKeyLabel', align: 'left', sortable: true },
  { name: 'requestCount', label: 'Requests', field: 'requestCount', align: 'right', sortable: true },
  { name: 'uptime', label: 'Uptime', field: 'createdAt', align: 'left', sortable: true },
  { name: 'lastRequest', label: 'Last Request', field: 'lastRequest', align: 'left', sortable: true },
];

function tunnelUrl(subdomain: string): string {
  const protocol = window.location.protocol;
  const baseDomain = window.location.host;
  return `${protocol}//${subdomain}.${baseDomain}`;
}

function formatUptime(createdAt: string): string {
  const created = new Date(createdAt);
  const now = new Date();
  const diff = Math.floor((now.getTime() - created.getTime()) / 1000);
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`;
  return `${Math.floor(diff / 86400)}d ${Math.floor((diff % 86400) / 3600)}h`;
}

function formatTimestamp(ts: string): string {
  const date = new Date(ts);
  return date.toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
</script>
