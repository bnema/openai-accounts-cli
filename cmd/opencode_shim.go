package cmd

const opencodeShim = `export const OAPlugin = async ({ client, $ }) => {
  return {
    "session.error": async (event) => {
      try {
        const result = await $` + "`" + `oa opencode handle --json` + "`" + `.stdin(JSON.stringify({
          provider: event.provider ?? "openai",
          status: event.status ?? 0,
          message: event.message ?? "",
          account_id: event.account_id ?? event.accountId ?? "",
        })).json()

        if (!event.metadata?.oaRetried && result.retry_safe && result.auth) {
			await client.auth.set({ path: { id: "openai" }, body: result.auth })
          return {
            retry: true,
            metadata: {
              ...(event.metadata ?? {}),
              oaRetried: true,
            },
          }
        }

		await client.tui.showToast({ body: { message: result.message, variant: "info" } })
		return { retry: false }
	} catch (error) {
		await client.tui.showToast({ body: { message: error?.message ?? "opencode handle failed", variant: "info" } })
		return { retry: false }
	}
    },
  }
}
`
