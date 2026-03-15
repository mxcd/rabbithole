<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated>
      <q-toolbar>
        <q-toolbar-title>
          <q-icon name="swap_horiz" class="q-mr-sm" />
          Rabbithole
        </q-toolbar-title>
        <q-space />
        <q-badge :color="connected ? 'positive' : 'negative'" :label="connected ? 'Live' : 'Disconnected'" />
      </q-toolbar>
    </q-header>

    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { useDashboard } from 'src/composables/useDashboard';

const { connected } = useDashboard();
</script>
