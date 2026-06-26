(() => {
  const CONVERSATIONS_KEY = "llm-rag.conversations";
  const ACTIVE_ID_KEY = "llm-rag.activeId";
  const TITLE_MAX_LEN = 28;

  const appShellEl = document.getElementById("app-shell");
  const sidebarToggleEl = document.getElementById("sidebar-toggle");
  const conversationListEl = document.getElementById("conversation-list");
  const conversationTitleEl = document.getElementById("conversation-title");
  const newChatBtn = document.getElementById("new-chat-btn");
  const messagesEl = document.getElementById("messages");
  const formEl = document.getElementById("chat-form");
  const inputEl = document.getElementById("message-input");
  const sendBtn = document.getElementById("send-btn");
  const sessionLabelEl = document.getElementById("session-label");

  // ---- ドキュメント管理 ----
  const tabChatEl = document.getElementById("tab-chat");
  const tabDocsEl = document.getElementById("tab-docs");
  const docsViewEl = document.getElementById("docs-view");
  const composerBarEl = document.querySelector(".composer-bar");
  const ragBadgeEl = document.getElementById("rag-badge");
  const docTitleEl = document.getElementById("doc-title");
  const docTextEl = document.getElementById("doc-text");
  const docsSubmitEl = document.getElementById("docs-submit");
  const docsStatusEl = document.getElementById("docs-status");
  const docsListEl = document.getElementById("docs-list");
  const docsRefreshEl = document.getElementById("docs-refresh");

  let currentView = "chat"; // "chat" | "docs"
  let docCount = 0;

  function switchView(view) {
    currentView = view;
    const isChat = view === "chat";

    tabChatEl.classList.toggle("active", isChat);
    tabDocsEl.classList.toggle("active", !isChat);

    messagesEl.hidden = !isChat;
    composerBarEl.hidden = !isChat;
    docsViewEl.hidden = isChat;

    if (isChat) {
      conversationTitleEl.textContent =
        findConversation(activeId)?.title || "新しい会話";
      sessionLabelEl.hidden = false;
    } else {
      conversationTitleEl.textContent = "ドキュメント管理";
      sessionLabelEl.hidden = true;
      loadDocuments();
    }
  }

  function setDocsStatus(msg, type) {
    docsStatusEl.textContent = msg;
    docsStatusEl.className = "docs-status" + (type ? " " + type : "");
  }

  async function ingestDocument() {
    const title = docTitleEl.value.trim();
    const text = docTextEl.value.trim();
    if (!text) {
      setDocsStatus("テキストを入力してください。", "error");
      return;
    }

    docsSubmitEl.disabled = true;
    setDocsStatus("登録中...");

    try {
      const res = await fetch("/api/documents", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, text }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);

      setDocsStatus(
        `登録完了 — ${data.chunks} チャンクに分割されました。`,
        "success"
      );
      docTitleEl.value = "";
      docTextEl.value = "";
      loadDocuments();
    } catch (err) {
      setDocsStatus(`エラー: ${err.message}`, "error");
    } finally {
      docsSubmitEl.disabled = false;
    }
  }

  async function loadDocuments() {
    docsListEl.innerHTML = '<li class="docs-list-empty">読み込み中...</li>';
    try {
      const res = await fetch("/api/documents");
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);

      const docs = data.documents || [];
      docCount = docs.length;
      updateRagBadge();
      renderDocumentList(docs);
    } catch (err) {
      docsListEl.innerHTML = `<li class="docs-list-empty">読み込みエラー: ${err.message}</li>`;
    }
  }

  function renderDocumentList(docs) {
    if (docs.length === 0) {
      docsListEl.innerHTML =
        '<li class="docs-list-empty">まだドキュメントが登録されていません。<br>上のフォームからテキストを登録してください。</li>';
      return;
    }

    docsListEl.innerHTML = "";
    docs.forEach((doc, i) => {
      const li = document.createElement("li");
      li.className = "docs-list-item";
      li.innerHTML = `
        <div class="docs-list-icon">📄</div>
        <div class="docs-list-body">
          <div class="docs-list-title">${escapeHtml(doc.title || "(タイトルなし)")}</div>
          <div class="docs-list-meta">ID: ${doc.id ? doc.id.slice(0, 8) : "—"}…</div>
        </div>
      `;
      docsListEl.appendChild(li);
    });
  }

  function updateRagBadge() {
    ragBadgeEl.textContent = docCount > 0 ? `文書 ${docCount} 件` : "";
  }

  function escapeHtml(str) {
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  tabChatEl.addEventListener("click", () => switchView("chat"));
  tabDocsEl.addEventListener("click", () => switchView("docs"));
  docsSubmitEl.addEventListener("click", ingestDocument);
  docsRefreshEl.addEventListener("click", loadDocuments);

  let conversations = loadConversations();
  let activeId = localStorage.getItem(ACTIVE_ID_KEY) || "";

  function loadConversations() {
    try {
      return JSON.parse(localStorage.getItem(CONVERSATIONS_KEY) || "[]");
    } catch {
      return [];
    }
  }

  function saveConversations() {
    localStorage.setItem(CONVERSATIONS_KEY, JSON.stringify(conversations));
  }

  function setActiveId(id) {
    activeId = id;
    localStorage.setItem(ACTIVE_ID_KEY, id);
  }

  function findConversation(id) {
    return conversations.find((c) => c.id === id) || null;
  }

  function titleFromMessage(message) {
    const trimmed = message.trim().replace(/\s+/g, " ");
    return trimmed.length > TITLE_MAX_LEN ? trimmed.slice(0, TITLE_MAX_LEN) + "…" : trimmed;
  }

  function closeSidebarOnMobile() {
    appShellEl.classList.remove("sidebar-open");
  }

  // ---- sidebar ----

  function renderSidebar() {
    conversationListEl.innerHTML = "";

    if (conversations.length === 0) {
      const empty = document.createElement("div");
      empty.className = "conversation-empty";
      empty.textContent = "まだ会話履歴がありません";
      conversationListEl.appendChild(empty);
      return;
    }

    for (const conv of conversations) {
      const item = document.createElement("div");
      item.className = "conversation-item" + (conv.id === activeId ? " active" : "");

      const title = document.createElement("span");
      title.className = "conversation-item-title";
      title.textContent = conv.title || "新しい会話";
      item.appendChild(title);

      const delBtn = document.createElement("button");
      delBtn.className = "conversation-item-delete";
      delBtn.type = "button";
      delBtn.setAttribute("aria-label", "この会話を削除");
      delBtn.textContent = "×";
      delBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        deleteConversation(conv.id);
      });
      item.appendChild(delBtn);

      item.addEventListener("click", () => selectConversation(conv.id));
      conversationListEl.appendChild(item);
    }
  }

  function selectConversation(id) {
    setActiveId(id);
    renderSidebar();
    renderActiveConversation();
    closeSidebarOnMobile();
  }

  function deleteConversation(id) {
    conversations = conversations.filter((c) => c.id !== id);
    saveConversations();

    if (activeId === id) {
      setActiveId(conversations.length > 0 ? conversations[0].id : "");
    }
    renderSidebar();
    renderActiveConversation();
  }

  function startNewConversation() {
    setActiveId("");
    renderSidebar();
    renderActiveConversation();
    closeSidebarOnMobile();
    inputEl.focus();
  }

  // ---- main pane ----

  function renderActiveConversation() {
    const conv = findConversation(activeId);
    conversationTitleEl.textContent = conv ? conv.title : "新しい会話";
    sessionLabelEl.textContent = conv ? `session: ${conv.id.slice(0, 8)}` : "";

    messagesEl.innerHTML = "";
    if (!conv || conv.messages.length === 0) {
      messagesEl.innerHTML = '<div class="empty-state"><p>こんにちは。何について話しましょうか?</p></div>';
      return;
    }
    for (const turn of conv.messages) {
      appendRow(turn.role, turn.content);
    }
  }

  function clearEmptyState() {
    const empty = messagesEl.querySelector(".empty-state");
    if (empty) empty.remove();
  }

  function assistantAvatar() {
    const avatar = document.createElement("div");
    avatar.className = "avatar";
    avatar.textContent = "L";
    return avatar;
  }

  function appendRow(role, content) {
    clearEmptyState();

    const row = document.createElement("div");
    row.className = `row ${role}`;

    const bubble = document.createElement("div");
    bubble.className = "bubble";

    if (content === null) {
      bubble.innerHTML = '<span class="typing"><span></span><span></span><span></span></span>';
    } else {
      bubble.textContent = content;
    }

    if (role === "assistant") {
      row.appendChild(assistantAvatar());
    }
    row.appendChild(bubble);

    messagesEl.appendChild(row);
    messagesEl.scrollTop = messagesEl.scrollHeight;
    return { row, bubble };
  }

  function autoResize() {
    inputEl.style.height = "auto";
    inputEl.style.height = Math.min(inputEl.scrollHeight, 160) + "px";
  }

  function setSending(sending) {
    sendBtn.disabled = sending;
    inputEl.disabled = sending;
  }

  function bumpToFront(id) {
    const idx = conversations.findIndex((c) => c.id === id);
    if (idx > 0) {
      const [conv] = conversations.splice(idx, 1);
      conversations.unshift(conv);
    }
  }

  async function sendMessage(message) {
    appendRow("user", message);
    setSending(true);
    const pending = appendRow("assistant", null);

    const isDraft = !activeId;
    const requestSessionId = isDraft ? undefined : activeId;

    try {
      const res = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ session_id: requestSessionId, message }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || `HTTP ${res.status}`);
      }

      pending.bubble.textContent = data.response;

      if (isDraft) {
        const conv = {
          id: data.session_id,
          title: titleFromMessage(message),
          messages: [
            { role: "user", content: message },
            { role: "assistant", content: data.response },
          ],
        };
        conversations.unshift(conv);
        setActiveId(conv.id);
      } else {
        const conv = findConversation(activeId);
        conv.messages.push({ role: "user", content: message });
        conv.messages.push({ role: "assistant", content: data.response });
        bumpToFront(activeId);
      }

      saveConversations();
      conversationTitleEl.textContent = findConversation(activeId).title;
      sessionLabelEl.textContent = `session: ${activeId.slice(0, 8)}`;
      renderSidebar();
    } catch (err) {
      pending.row.className = "row error";
      pending.bubble.textContent = `エラー: ${err.message}`;
    } finally {
      setSending(false);
      inputEl.focus();
    }
  }

  formEl.addEventListener("submit", (e) => {
    e.preventDefault();
    const message = inputEl.value.trim();
    if (!message) return;
    inputEl.value = "";
    autoResize();
    sendMessage(message);
  });

  inputEl.addEventListener("input", autoResize);

  inputEl.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      formEl.requestSubmit();
    }
  });

  newChatBtn.addEventListener("click", startNewConversation);
  sidebarToggleEl.addEventListener("click", () => {
    appShellEl.classList.toggle("sidebar-open");
  });

  if (activeId && !findConversation(activeId)) {
    setActiveId(conversations.length > 0 ? conversations[0].id : "");
  }

  renderSidebar();
  renderActiveConversation();

  // 起動時にドキュメント数をバッジ表示するため件数だけ取得する
  fetch("/api/documents")
    .then((r) => r.json())
    .then((data) => {
      docCount = (data.documents || []).length;
      updateRagBadge();
    })
    .catch(() => {});
})();
