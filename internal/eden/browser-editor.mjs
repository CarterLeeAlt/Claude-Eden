const origin = "http://127.0.0.1:18080";
const cookieName = "code-server-session";

// Session cookies can be absent after Chromium restores cached editor pages.
// Confirm authentication before reusing those documents or declaring them ready.
export async function authenticateEditor(cdp, password, { request = fetch, signal = AbortSignal.timeout(5000) } = {}) {
  // The default cookie store is available before restored renderers wake up.
  const { cookies } = await cdp("Storage.getCookies");
  const cookie = cookies.find(item => item.name === cookieName && item.domain === "127.0.0.1" && item.path === "/");
  if (cookie) {
    const response = await request(origin + "/login", {
      redirect: "manual", headers: { Cookie: `${cookie.name}=${cookie.value}` }, signal,
    });
    await response.body?.cancel();
    if (response.status === 302) return false;
    if (response.status !== 200) throw new Error("Editor authentication check failed");
  }
  const response = await request(origin + "/login", {
    method: "POST", redirect: "manual", headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({ password }), signal,
  });
  const received = response.headers.getSetCookie().map(raw => raw.split(";")[0]).filter(raw => raw.startsWith(cookieName + "="));
  await response.body?.cancel();
  if (response.status !== 302 || received.length !== 1 || received[0] === cookieName + "=")
    throw new Error("Editor authentication is unavailable");
  await cdp("Storage.setCookies", { cookies: [{
    name: cookieName, value: received[0].slice(cookieName.length + 1),
    url: origin + "/", path: "/", httpOnly: true, sameSite: "Lax",
  }] });
  return true;
}
