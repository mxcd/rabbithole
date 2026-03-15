import type { RouteRecordRaw } from 'vue-router';

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    component: () => import('layouts/BlankLayout.vue'),
    children: [
      { path: '', component: () => import('pages/LoginPage.vue') },
    ],
  },
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', component: () => import('pages/DashboardPage.vue') },
    ],
  },
  {
    path: '/:catchAll(.*)*',
    component: () => import('layouts/BlankLayout.vue'),
    children: [
      { path: '', component: () => import('pages/ErrorNotFound.vue') },
    ],
  },
];

export default routes;
