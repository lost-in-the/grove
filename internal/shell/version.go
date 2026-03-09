package shell

// ShellVersion is incremented when the shell integration template changes in
// a way that requires users to re-source it. The grove binary embeds this
// version in the generated shell code and checks it at runtime.
const ShellVersion = 4
