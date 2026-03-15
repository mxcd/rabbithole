import { defineConfig } from '#q-app/wrappers';

export default defineConfig(() => {
  return {
    boot: [],

    css: ['app.scss'],

    extras: [
      'roboto-font',
      'material-icons',
    ],

    build: {
      target: {
        browser: ['es2022', 'firefox115', 'chrome115', 'safari14'],
        node: 'node20',
      },

      typescript: {
        strict: true,
        vueShim: true,
      },

      vueRouterMode: 'history',
    },

    devServer: {
      open: false,
    },

    framework: {
      config: {},
      plugins: ['Notify'],
    },

    animations: [],
  };
});
