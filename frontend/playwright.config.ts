import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: 'http://localhost:4100',
    headless: true,
  },
  projects: [
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: [
    {
      command: 'cd .. && bin/mock-provider -port 4101',
      port: 4101,
      reuseExistingServer: true,
      timeout: 10_000,
    },
    {
      command: 'cd .. && bin/mora web --debug --demo -p 4100 -c e2e/mora-test.conf',
      port: 4100,
      reuseExistingServer: true,
      timeout: 30_000,
    },
  ],
})
