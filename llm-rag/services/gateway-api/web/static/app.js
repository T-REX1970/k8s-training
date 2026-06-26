(() => {
  const CONVERSATIONS_KEY = "llm-rag.conversations";
  const ACTIVE_ID_KEY = "llm-rag.activeId";
  const TITLE_MAX_LEN = 28;

  // marked.js 設定: GFM有効、改行を<br>に変換
  if (typeof marked !== "undefined") {
    marked.setOptions({ gfm: true, breaks: true });
  }

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

  let currentView = "chat";
  let docCount = 0;

  // ---- Markdown レンダリング ----

  function renderMarkdown(text) {
    if (typeof marked !== "undefined" && typeof DOMPurify !== "undefined") {
      return DOMPurify.sanitize(marked.parse(text));
    }
    // フォールバック: エスケープしてプレーンテキスト表示
    return escapeHtml(text).replace(/\n/g, "<br>");
  }

  function attachCopyButtons(container) {
    container.querySelectorAll("pre").forEach(pre => {
      if (pre.querySelector(".copy-btn")) return;
      const btn = document.createElement("button");
      btn.className = "copy-btn";
      btn.textContent = "コピー";
      btn.addEventListener("click", () => {
        const code = pre.querySelector("code");
        navigator.clipboard.writeText(code ? code.innerText : pre.innerText).then(() => {
          btn.textContent = "✓ コピー済み";
          setTimeout(() => { btn.textContent = "コピー"; }, 1500);
        });
      });
      pre.appendChild(btn);
    });
  }

  // ---- View 切り替え ----

  function switchView(view) {
    currentView = view;
    const isChat = view === "chat";
    tabChatEl.classList.toggle("active", isChat);
    tabDocsEl.classList.toggle("active", !isChat);
    messagesEl.hidden = !isChat;
    composerBarEl.hidden = !isChat;
    docsViewEl.hidden = isChat;

    if (isChat) {
      conversationTitleEl.textContent = findConversation(activeId)?.title || "新しい会話";
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

  // ---- ドキュメント登録 ----

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
      setDocsStatus(`登録完了 — ${data.chunks} チャンクに分割されました。`, "success");
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
    docs.forEach(doc => {
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

  // ---- ファイルドラッグ&ドロップ ----

  // ドキュメントビュー上へのドロップで .txt / .md を自動入力する
  docsViewEl.addEventListener("dragover", e => {
    e.preventDefault();
    docsViewEl.classList.add("drag-over");
  });
  docsViewEl.addEventListener("dragleave", e => {
    // 子要素への移動は dragleave に見せかけてしまうため relatedTarget で判定
    if (!docsViewEl.contains(e.relatedTarget)) {
      docsViewEl.classList.remove("drag-over");
    }
  });
  docsViewEl.addEventListener("drop", e => {
    e.preventDefault();
    docsViewEl.classList.remove("drag-over");
    const file = e.dataTransfer.files[0];
    if (!file) return;
    const isText = file.type.startsWith("text/") ||
      file.name.endsWith(".txt") || file.name.endsWith(".md");
    if (!isText) {
      setDocsStatus("テキストファイル (.txt, .md) のみ対応しています。", "error");
      return;
    }
    file.text().then(text => {
      docTextEl.value = text;
      if (!docTitleEl.value) {
        docTitleEl.value = file.name.replace(/\.(txt|md)$/, "");
      }
      setDocsStatus(`"${file.name}" を読み込みました。登録ボタンを押してください。`, "success");
    });
  });

  tabChatEl.addEventListener("click", () => switchView("chat"));
  tabDocsEl.addEventListener("click", () => switchView("docs"));
  docsSubmitEl.addEventListener("click", ingestDocument);
  docsRefreshEl.addEventListener("click", loadDocuments);

  // ---- 会話管理 ----

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
    return conversations.find(c => c.id === id) || null;
  }

  function titleFromMessage(message) {
    const trimmed = message.trim().replace(/\s+/g, " ");
    return trimmed.length > TITLE_MAX_LEN ? trimmed.slice(0, TITLE_MAX_LEN) + "…" : trimmed;
  }

  function closeSidebarOnMobile() {
    appShellEl.classList.remove("sidebar-open");
  }

  // ---- サイドバー ----

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
      delBtn.addEventListener("click", e => {
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
    conversations = conversations.filter(c => c.id !== id);
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

  // ---- メッセージ表示 ----

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
      const { bubble } = appendRow(turn.role, turn.content);
      if (turn.role === "assistant" && turn.sources && turn.sources.length > 0) {
        // 行の row 要素を取得して sources を付ける
        const row = bubble.closest(".row");
        if (row) appendSourcesPanel(row, turn.sources);
      }
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

  // content=null のとき typing アニメーション、文字列のとき Markdown レンダリング
  function appendRow(role, content) {
    clearEmptyState();
    const row = document.createElement("div");
    row.className = `row ${role}`;
    const bubble = document.createElement("div");
    bubble.className = "bubble";

    if (content === null) {
      bubble.innerHTML = '<span class="typing"><span></span><span></span><span></span></span>';
    } else {
      bubble.innerHTML = renderMarkdown(content);
      attachCopyButtons(bubble);
    }

    if (role === "assistant") row.appendChild(assistantAvatar());
    row.appendChild(bubble);
    messagesEl.appendChild(row);
    messagesEl.scrollTop = messagesEl.scrollHeight;
    return { row, bubble };
  }

  // ストリーミング中はバブル内容を逐次更新しつつカーソルを表示する
  function updateStreamingBubble(bubble, text, done) {
    bubble.innerHTML = renderMarkdown(text);
    if (!done) {
      const cursor = document.createElement("span");
      cursor.className = "streaming-cursor";
      bubble.appendChild(cursor);
    } else {
      attachCopyButtons(bubble);
    }
    messagesEl.scrollTop = messagesEl.scrollHeight;
  }

  // RAG ソース引用パネルを row の下に追加する
  function appendSourcesPanel(row, sources) {
    if (!sources || sources.length === 0) return;
    const panel = document.createElement("div");
    panel.className = "sources-panel";

    const toggle = document.createElement("button");
    toggle.className = "sources-toggle";
    toggle.innerHTML = `<span class="arrow">▶</span> 参照ソース (${sources.length} 件)`;

    const list = document.createElement("div");
    list.className = "sources-list";

    sources.forEach(src => {
      const item = document.createElement("div");
      item.className = "source-item";
      item.innerHTML = `
        <span class="source-icon">📄</span>
        <span class="source-title">${escapeHtml(src.title || src.doc_id || "—")}</span>
        <span class="source-score">${(src.score * 100).toFixed(0)}%</span>
      `;
      list.appendChild(item);
    });

    toggle.addEventListener("click", () => {
      const open = list.classList.toggle("visible");
      toggle.classList.toggle("open", open);
    });

    panel.appendChild(toggle);
    panel.appendChild(list);
    row.appendChild(panel);
  }

  // ---- テキストエリア自動リサイズ ----

  function autoResize() {
    inputEl.style.height = "auto";
    inputEl.style.height = Math.min(inputEl.scrollHeight, 160) + "px";
  }

  function setSending(sending) {
    sendBtn.disabled = sending;
    inputEl.disabled = sending;
  }

  function bumpToFront(id) {
    const idx = conversations.findIndex(c => c.id === id);
    if (idx > 0) {
      const [conv] = conversations.splice(idx, 1);
      conversations.unshift(conv);
    }
  }

  // ---- SSE ストリーミング送信 ----

  async function sendMessage(message) {
    appendRow("user", message);
    setSending(true);
    const { row: pendingRow, bubble: pendingBubble } = appendRow("assistant", null);

    const isDraft = !activeId;
    const requestSessionId = isDraft ? undefined : activeId;

    let fullText = "";
    let sessionId = requestSessionId;
    let sources = null;

    try {
      const res = await fetch("/api/chat/stream", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ session_id: requestSessionId, message }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${res.status}`);
      }

      // typing インジケーターを除去してストリーミング開始
      pendingBubble.innerHTML = "";

      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";

      outer: while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buf += decoder.decode(value, { stream: true });

        // SSE は "\n\n" 区切り
        const parts = buf.split("\n\n");
        buf = parts.pop(); // 最後の未完成チャンクはバッファに残す

        for (const part of parts) {
          const line = part.trim();
          if (!line.startsWith("data: ")) continue;
          const payload = line.slice("data: ".length);
          if (payload === "[DONE]") break outer;

          let evt;
          try { evt = JSON.parse(payload); } catch { continue; }

          if (evt.session_id) {
            sessionId = evt.session_id;
          } else if (evt.token !== undefined) {
            fullText += evt.token;
            updateStreamingBubble(pendingBubble, fullText, false);
          } else if (evt.sources) {
            sources = evt.sources;
          } else if (evt.error) {
            throw new Error(evt.error);
          }
        }
      }

      // ストリーム完了: カーソル除去・コピーボタン付与
      updateStreamingBubble(pendingBubble, fullText, true);

      // ソース引用パネル表示
      if (sources && sources.length > 0) {
        appendSourcesPanel(pendingRow, sources);
      }

      // 会話履歴を更新
      if (isDraft && sessionId) {
        const conv = {
          id: sessionId,
          title: titleFromMessage(message),
          messages: [
            { role: "user", content: message },
            { role: "assistant", content: fullText, sources },
          ],
        };
        conversations.unshift(conv);
        setActiveId(conv.id);
      } else if (activeId) {
        const conv = findConversation(activeId);
        if (conv) {
          conv.messages.push({ role: "user", content: message });
          conv.messages.push({ role: "assistant", content: fullText, sources });
          bumpToFront(activeId);
        }
      }

      saveConversations();
      const activeConv = findConversation(activeId);
      if (activeConv) conversationTitleEl.textContent = activeConv.title;
      sessionLabelEl.textContent = `session: ${activeId.slice(0, 8)}`;
      renderSidebar();

    } catch (err) {
      pendingRow.className = "row error";
      pendingBubble.textContent = `エラー: ${err.message}`;
    } finally {
      setSending(false);
      inputEl.focus();
    }
  }

  // ---- フォームイベント ----

  formEl.addEventListener("submit", e => {
    e.preventDefault();
    const message = inputEl.value.trim();
    if (!message) return;
    inputEl.value = "";
    autoResize();
    sendMessage(message);
  });

  inputEl.addEventListener("input", autoResize);

  inputEl.addEventListener("keydown", e => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      formEl.requestSubmit();
    }
  });

  newChatBtn.addEventListener("click", startNewConversation);
  sidebarToggleEl.addEventListener("click", () => {
    appShellEl.classList.toggle("sidebar-open");
  });

  // ---- ユーティリティ ----

  function escapeHtml(str) {
    return str
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  // ---- 初期化 ----

  if (activeId && !findConversation(activeId)) {
    setActiveId(conversations.length > 0 ? conversations[0].id : "");
  }

  renderSidebar();
  renderActiveConversation();

  // 起動時にドキュメント数バッジだけ更新
  fetch("/api/documents")
    .then(r => r.json())
    .then(data => {
      docCount = (data.documents || []).length;
      updateRagBadge();
    })
    .catch(() => {});
})();
