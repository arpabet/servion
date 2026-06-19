# servion-vue-example
Example of using servion project

Create webapp from the scratch
```
node -v
npm create vue@latest webapp
cd webapp
npm run build
```

Edit the file 'vite.config.js'
```
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  base: './', // Ensures correct asset paths for static deployment
  build: {
    outDir: '../assets', // Output directory
  },
});
```

Create file 'router/index.js'
```
import { createRouter, createWebHashHistory } from 'vue-router';
import App from '../App.vue';

const router = createRouter({
    history: createWebHashHistory(), // Hash-based navigation
    routes: [
        { path: '/', component: App },
    ],
});

export default router;
```

Use existing application
```
cd webapp
npm install
npm run build
```
