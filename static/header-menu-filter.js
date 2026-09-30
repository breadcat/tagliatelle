document.addEventListener('DOMContentLoaded', function () {
  document.querySelectorAll('.tag-filter-input').forEach(function (input) {
    var ul = input.closest('ul');
    var items = Array.from(ul.querySelectorAll('.tag-filter-item'));
    var more = ul.querySelector('.tag-filter-more');
    input.addEventListener('input', function () {
      var q = input.value.trim().toLowerCase();
      var matches = q ? items.filter(function (li) {
        return li.dataset.name.toLowerCase().includes(q);
      }) : [];
      var show = q !== '' && matches.length < 20;
      items.forEach(function (li) { li.style.display = 'none'; });
      if (show) matches.forEach(function (li) { li.style.display = ''; });
      more.style.display = show ? 'none' : '';
    });
  });
});
