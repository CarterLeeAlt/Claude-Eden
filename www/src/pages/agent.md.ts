import guide from '../../../docs/guide/agent.md?raw';

export function GET() {
  const text = guide.replace(/^---\n[\s\S]*?\n---\n/, '').replace(/\]\(([a-z-]+)\.md(#[^)]*)?\)/g, (_, slug: string, hash = '') => `](https://github.com/CarterLeeAlt/Claude-Eden/blob/main/docs/guide/${slug}.md${hash})`);
  return new Response(text, { headers: { 'Content-Type': 'text/markdown; charset=utf-8' } });
}
