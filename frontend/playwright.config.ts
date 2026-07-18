import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:4000',
    headless: true,
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: 'cd .. && bin/mora web --debug --demo -c e2e/mora-test.conf',
    port: 4000,
    reuseExistingServer: true,
    timeout: 30_000,
  },
})
