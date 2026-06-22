(() => {
  const SESSION_KEY = "llm-rag.session_id";
  const HISTORY_KEY = "llm-rag.history";

  const messagesEl = document.getElementById("messages");
  const formEl = document.getElementById("chat-form");
  const inputEl = document.getElementById("message-input");
  const sessionLabelEl = document.getElementById("session-label");
  const resetBtn = document.getElementById("reset-btn");

  let sessionId = localStorage.getItem(SESSION_KEY) || "";
  let history = loadHistory();

  function loadHistory() {
    try {
      return JSON.parse(localStorage.getItem(HISTORY_KEY) || "[]");
    } catch {
      return [];
    }
  }

  function saveHistory() {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
  }

  function renderSessionLabel() {
    sessionLabelEl.textContent = sessionId ? `session: ${sessionId.slice(0, 8)}` : "";
  }

  function appendBubble(role, text) {
    const bubble = document.createElement("div");
    bubble.className = `bubble ${role}`;
    bubble.textContent = text;
    messagesEl.appendChild(bubble);
    messagesEl.scrollTop = messagesEl.scrollHeight;
    return bubble;
  }

  function renderHistory() {
    messagesEl.innerHTML = "";
    for (const turn of history) {
      appendBubble(turn.role, turn.content);
    }
  }

  async function sendMessage(message) {
    appendBubble("user", message);
    history.push({ role: "user", content: message });
    saveHistory();

    const pending = appendBubble("pending", "考え中...");

    try {
      const res = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ session_id: sessionId || undefined, message }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}`);
      }

      sessionId = data.session_id;
      localStorage.setItem(SESSION_KEY, sessionId);
      renderSessionLabel();

      pending.className = "bubble assistant";
      pending.textContent = data.response;
      history.push({ role: "assistant", content: data.response });
      saveHistory();
    } catch (err) {
      pending.className = "bubble error";
      pending.textContent = `エラー: ${err.message}`;
    }
  }

  formEl.addEventListener("submit", (e) => {
    e.preventDefault();
    const message = inputEl.value.trim();
    if (!message) return;
    inputEl.value = "";
    sendMessage(message);
  });

  resetBtn.addEventListener("click", () => {
    sessionId = "";
    history = [];
    localStorage.removeItem(SESSION_KEY);
    localStorage.removeItem(HISTORY_KEY);
    renderSessionLabel();
    renderHistory();
  });

  renderSessionLabel();
  renderHistory();
})();
