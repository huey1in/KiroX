// ===== 通用自定义下拉组件（.custom-dropdown）=====
// 用法：
//   <div class="custom-dropdown" data-onchange="回调函数名">
//     <div class="dropdown-selected" onclick="toggleDropdown(this)">
//       <span class="dropdown-selected-text">当前值</span>
//       <span class="dropdown-arrow"></span>
//     </div>
//     <div class="dropdown-options">
//       <div class="dropdown-option" data-value="x">选项x</div>
//     </div>
//   </div>
// 选中后读取容器 el.dataset.value；回调函数签名 (value, label)。

function toggleDropdown(triggerEl, ev) {
  if (ev && typeof ev.stopPropagation === 'function') ev.stopPropagation();
  var wrap = triggerEl.closest('.custom-dropdown');
  if (!wrap) return;
  var wasOpen = wrap.classList.contains('open');
  closeAllDropdowns();
  if (!wasOpen) {
    wrap.classList.add('open');
    var t = wrap.querySelector('.dropdown-selected');
    if (t) t.classList.add('active');
    var o = wrap.querySelector('.dropdown-options');
    if (o) {
      // 用 fixed 定位（对齐 selected 视口坐标），避免被 modal 的 overflow 裁剪
      var r = (t || wrap).getBoundingClientRect();
      o.style.position = 'fixed';
      o.style.top = (r.bottom + 4) + 'px';
      o.style.left = r.left + 'px';
      o.style.width = r.width + 'px';
      o.style.right = 'auto';
      o.style.maxHeight = 'min(320px, calc(100vh - ' + (r.bottom + 8) + 'px))';
      o.style.overflowY = 'auto';
      o.classList.add('show');
    }
  }
}

function selectDropdownOption(el, ev) {
  if (ev && typeof ev.stopPropagation === 'function') ev.stopPropagation();
  var wrap = el.closest('.custom-dropdown');
  if (!wrap) return;
  var val = el.getAttribute('data-value') || '';
  var label = (el.textContent || '').trim();
  wrap.dataset.value = val;
  var txt = wrap.querySelector('.dropdown-selected-text');
  if (txt) txt.textContent = label;
  wrap.querySelectorAll('.dropdown-option').forEach(function(o) {
    o.classList.toggle('selected', o === el);
  });
  closeAllDropdowns();
  var fnName = wrap.getAttribute('data-onchange');
  if (fnName) {
    var fn = window[fnName];
    if (typeof fn === 'function') fn(val, label);
  }
}

function closeAllDropdowns() {
  document.querySelectorAll('.custom-dropdown.open').forEach(function(c) {
    c.classList.remove('open');
    var t = c.querySelector('.dropdown-selected');
    if (t) t.classList.remove('active');
    var o = c.querySelector('.dropdown-options');
    if (o) {
      o.classList.remove('show');
      // 清理 fixed 定位内联样式，还原为相对定位默认
      o.style.position = '';
      o.style.top = '';
      o.style.left = '';
      o.style.width = '';
      o.style.right = '';
      o.style.maxHeight = '';
      o.style.overflowY = '';
    }
  });
}

// 事件委托：点击 option 选择；点击 dropdown 外部关闭
document.addEventListener('click', function(e) {
  var t = e.target;
  var option = t && t.closest ? t.closest('.dropdown-option') : null;
  if (option) { selectDropdownOption(option, e); return; }
  if (!(t && t.closest) || !t.closest('.custom-dropdown')) closeAllDropdowns();
});

// 设置某个 custom-dropdown 的选中值（val 匹配 data-value，找不到则忽略）
function setDropdownValue(wrap, val) {
  if (!wrap) return;
  var opt = wrap.querySelector('.dropdown-option[data-value="' + val + '"]');
  wrap.dataset.value = val;
  var txt = wrap.querySelector('.dropdown-selected-text');
  if (opt) {
    if (txt) txt.textContent = (opt.textContent || '').trim();
  } else {
    if (txt && val !== '') txt.textContent = val;
  }
  wrap.querySelectorAll('.dropdown-option').forEach(function(o) {
    o.classList.toggle('selected', o.getAttribute('data-value') === val);
  });
}
