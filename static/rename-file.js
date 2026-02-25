document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll(".rename-button").forEach(button => {
    button.addEventListener("click", () => {
      const fileID = button.dataset.fileId;
      const currentName = button.dataset.currentName;

      let newName = prompt("Enter new filename:", currentName);

      if (!newName) {
        return;
      }

      const lastDot = currentName.lastIndexOf('.');
      const currentExt = lastDot !== -1 ? currentName.slice(lastDot) : '';

      // If newName has no extension and the original did, append it
      if (currentExt && !newName.includes('.')) {
        newName = newName + currentExt;
      }

      const form = document.getElementById(`renameForm-${fileID}`);
      form.querySelector('input[name="newfilename"]').value = newName;
      form.submit();
    });
  });
});
