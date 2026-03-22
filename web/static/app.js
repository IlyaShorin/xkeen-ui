(() => {
  let editorState = null;

  const syncGlobalDirtyState = () => {
    const dirty = Boolean(editorState && editorState.isDirty());
    window.onbeforeunload = dirty ? () => true : null;
  };

  const initializeEditor = (root = document) => {
    const form = root.querySelector("[data-editor-form]");
    if (!form) {
      editorState = null;
      syncGlobalDirtyState();
      return;
    }

    const textarea = form.querySelector("textarea[name='content']");
    const saveButton = form.querySelector("[data-save-button]");
    const formatButton = form.querySelector("[data-format-json]");
    const message = form.querySelector("[data-editor-message]");

    if (!textarea || !saveButton || !formatButton || !message) {
      editorState = null;
      syncGlobalDirtyState();
      return;
    }

    const initialValue = textarea.value;
    const setMessage = (text, tone = "") => {
      message.hidden = !text;
      message.textContent = text;
      message.classList.remove("is-error", "is-success");
      if (tone) {
        message.classList.add(tone);
      }
    };

    const syncState = () => {
      const dirty = textarea.value !== initialValue;
      saveButton.disabled = !dirty;
      syncGlobalDirtyState();
    };

    textarea.addEventListener("input", syncState);
    formatButton.addEventListener("click", () => {
      try {
        const parsed = JSON.parse(textarea.value);
        const formatted = JSON.stringify(parsed, null, 2);
        textarea.value = `${formatted}\n`;
        textarea.dispatchEvent(new Event("input", { bubbles: true }));
        setMessage("JSON formatted.", "is-success");
      } catch (error) {
        const reason = error instanceof Error ? error.message : "Unknown JSON error";
        setMessage(`Format failed: ${reason}`, "is-error");
      }
    });

    editorState = {
      form,
      isDirty: () => textarea.value !== initialValue,
    };
    setMessage("");
    syncState();
  };

  const confirmDirtyNavigation = (event) => {
    if (!editorState || !editorState.isDirty()) {
      return;
    }

    const source = event.detail?.elt;
    if (source && editorState.form.contains(source)) {
      return;
    }

    if (!window.confirm("There are unsaved config changes. Continue?")) {
      event.preventDefault();
    }
  };

  document.addEventListener("DOMContentLoaded", () => initializeEditor(document));
  document.addEventListener("htmx:load", (event) => initializeEditor(event.target));
  document.addEventListener("htmx:beforeRequest", confirmDirtyNavigation);
})();
