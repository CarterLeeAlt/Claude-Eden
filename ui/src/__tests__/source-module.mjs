import ts from "typescript";

// VM fixtures still exercise the real business module, with only imports mocked.
export function transpileSource(source) {
  return ts.transpileModule(source, {
    compilerOptions: {
      target: ts.ScriptTarget.ES2023,
      module: ts.ModuleKind.ESNext,
    },
  }).outputText;
}
