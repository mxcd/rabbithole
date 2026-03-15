import { ref, onMounted, onUnmounted } from 'vue';

export interface TunnelInfo {
  subdomain: string;
  apiKeyLabel: string;
  createdAt: string;
  requestCount: number;
  lastRequest: string;
}

const tunnels = ref<TunnelInfo[]>([]);
const connected = ref(false);
let ws: WebSocket | null = null;
let refreshInterval: ReturnType<typeof setInterval> | null = null;

async function fetchTunnels() {
  try {
    const resp = await fetch('/api/v1/tunnels');
    const data = await resp.json();
    tunnels.value = data.tunnels ?? [];
  } catch {
    tunnels.value = [];
  }
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${window.location.host}/api/v1/dashboard/ws`;

  ws = new WebSocket(url);

  ws.onopen = () => {
    connected.value = true;
  };

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data);
      if (data.type === 'tunnels') {
        tunnels.value = data.tunnels ?? [];
      }
    } catch {
      // ignore parse errors
    }
  };

  ws.onclose = () => {
    connected.value = false;
    setTimeout(connectWebSocket, 3000);
  };

  ws.onerror = () => {
    ws?.close();
  };
}

export function useDashboard() {
  onMounted(() => {
    fetchTunnels();
    connectWebSocket();
    refreshInterval = setInterval(fetchTunnels, 5000);
  });

  onUnmounted(() => {
    if (refreshInterval) {
      clearInterval(refreshInterval);
    }
  });

  return {
    tunnels,
    connected,
    refresh: fetchTunnels,
  };
}
