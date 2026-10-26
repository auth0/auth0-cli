// An error the CLI reports with a stable code. The code is what --json prints as
// "error", and fixes are the commands or steps that resolve it.
export class PluginError extends Error {
  constructor(
    message: string,
    readonly code: string,
    readonly fixes?: string[],
  ) {
    super(message);
    this.name = new.target.name;
  }
}
