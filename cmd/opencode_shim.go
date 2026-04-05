package cmd

const opencodeShim = `import { tool } from "@opencode-ai/plugin"

export const OAPlugin = async ({ client, $ }) => {
  return {
    tool: {
      "oa-sync": tool({
        description: "Sync OpenCode auth with oa opencode sync",
        args: {},
        async execute() {
          try {
            const message = (await $` + "`" + `oa opencode sync` + "`" + `.quiet().text()).trim() || "Synced OpenCode auth"
            await client.tui.showToast({ body: { message, variant: "info" } })
            return message
          } catch (error) {
            const message = error?.stderr?.toString?.().trim?.() || error?.message || "oa opencode sync failed"
            await client.tui.showToast({ body: { message, variant: "error" } })
            return message
          }
        },
      }),
    },
  }
}
`
