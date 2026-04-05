package cmd

const opencodeShim = `export async function handle() {
  await $` + "`" + `oa opencode handle --json` + "`" + `
}
`
