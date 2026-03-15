<template>
  <q-page class="flex flex-center">
    <q-card style="min-width: 350px">
      <q-card-section class="text-center">
        <q-icon name="swap_horiz" size="48px" color="primary" />
        <div class="text-h5 q-mt-sm">Rabbithole</div>
        <div class="text-caption text-grey">Dashboard Login</div>
      </q-card-section>

      <q-card-section>
        <q-form @submit="handleLogin">
          <q-input
            v-model="password"
            type="password"
            label="Password"
            outlined
            autofocus
            :error="!!error"
            :error-message="error"
            @update:model-value="error = ''"
          />
          <q-btn
            type="submit"
            color="primary"
            label="Sign in"
            class="full-width q-mt-md"
            :loading="loading"
          />
        </q-form>
      </q-card-section>
    </q-card>
  </q-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { useRouter } from 'vue-router';

const router = useRouter();
const password = ref('');
const error = ref('');
const loading = ref(false);

async function handleLogin() {
  loading.value = true;
  error.value = '';

  try {
    const resp = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ identifier: 'admin', password: password.value }),
    });

    if (!resp.ok) {
      error.value = 'Invalid password';
      return;
    }

    router.push('/');
  } catch {
    error.value = 'Connection error';
  } finally {
    loading.value = false;
  }
}
</script>
