// PocketSub フロントエンド処理

// カテゴリごとのバッジ配色
const CATEGORY_BADGE_CLASS = {
	"エンタメ": "bg-rose-100 text-rose-700",
	"音楽": "bg-indigo-100 text-indigo-700",
	"アプリ": "bg-teal-100 text-teal-700",
	"ツール": "bg-orange-100 text-orange-700",
	"アーティスト": "bg-fuchsia-100 text-fuchsia-700",
	"その他": "bg-gray-100 text-gray-700",
};

// アイコン推測に失敗した場合のカテゴリ別フォールバック絵文字
const CATEGORY_FALLBACK_ICON = {
	"エンタメ": "🎬",
	"音楽": "🎵",
	"アプリ": "📱",
	"ツール": "🛠️",
	"アーティスト": "⭐",
	"その他": "📦",
};

// サービス名の部分一致による簡易アイコンマッピング
const ICON_RULES = [
	{ pattern: /netflix|hulu|amazon prime|disney/i, icon: "🎬" },
	{ pattern: /spotify|apple music|youtube music|amazon music/i, icon: "🎵" },
	{ pattern: /chatgpt|claude|copilot|gemini/i, icon: "🤖" },
	{ pattern: /icloud|google one|dropbox|drive/i, icon: "☁️" },
	{ pattern: /adobe|canva|figma/i, icon: "🎨" },
	{ pattern: /github|notion|slack|zoom/i, icon: "🛠️" },
];

function guessIcon(name, category) {
	const matched = ICON_RULES.find((rule) => rule.pattern.test(name));
	if (matched) return matched.icon;
	return CATEGORY_FALLBACK_ICON[category] ?? "📦";
}

// DOM要素の参照
const listEl = document.getElementById("subscription-list");
const emptyMessageEl = document.getElementById("empty-message");
const totalAnnualCostEl = document.getElementById("total-annual-cost");
const monthlyEquivalentCostEl = document.getElementById("monthly-equivalent-cost");
const subscriptionCountEl = document.getElementById("subscription-count");

const modalOverlayEl = document.getElementById("modal-overlay");
const modalTitleEl = document.getElementById("modal-title");
const formEl = document.getElementById("subscription-form");
const submitButtonEl = document.getElementById("submit-button");

const deleteModalOverlayEl = document.getElementById("delete-modal-overlay");
const deleteTargetNameEl = document.getElementById("delete-target-name");
const deleteCancelButtonEl = document.getElementById("delete-cancel-button");
const deleteConfirmButtonEl = document.getElementById("delete-confirm-button");

const aiAdviceButtonEl = document.getElementById("ai-advice-button");
const aiModalOverlayEl = document.getElementById("ai-modal-overlay");
const aiModalContentEl = document.getElementById("ai-modal-content");
const aiModalCloseButtonEl = document.getElementById("ai-modal-close-button");

const searchInputEl = document.getElementById("search-input");
const sortSelectEl = document.getElementById("sort-select");
const categoryTabsEl = document.getElementById("category-tabs");

const toastContainerEl = document.getElementById("toast-container");

// アプリの状態（取得済み全件データ・検索/フィルター/並び替え・削除対象）
let allSubscriptions = [];
let currentSearch = "";
let currentCategory = "all";
let currentSort = "renewal";
let deleteTarget = null;

// --- モーダル共通処理 ---

function showModal(overlay) {
	overlay.classList.remove("hidden");
	overlay.classList.add("flex");
}

function hideModal(overlay) {
	overlay.classList.add("hidden");
	overlay.classList.remove("flex");
}

// オーバーレイの背景クリックで閉じる
[modalOverlayEl, deleteModalOverlayEl, aiModalOverlayEl].forEach((overlay) => {
	overlay.addEventListener("click", (event) => {
		if (event.target === overlay) hideModal(overlay);
	});
});

function todayString() {
	const now = new Date();
	const yyyy = now.getFullYear();
	const mm = String(now.getMonth() + 1).padStart(2, "0");
	const dd = String(now.getDate()).padStart(2, "0");
	return `${yyyy}-${mm}-${dd}`;
}

// --- 追加・編集モーダル ---

document.getElementById("open-modal-button").addEventListener("click", openAddModal);
document.getElementById("close-modal-button").addEventListener("click", closeModal);

function openAddModal() {
	formEl.reset();
	delete formEl.dataset.editingId;
	modalTitleEl.textContent = "サブスクを追加";
	submitButtonEl.textContent = "追加する";
	formEl.elements["next_billing_date"].value = todayString();
	showModal(modalOverlayEl);
}

function openEditModal(subscription) {
	formEl.reset();
	formEl.dataset.editingId = subscription.id;
	modalTitleEl.textContent = "サブスクを編集";
	submitButtonEl.textContent = "保存する";
	formEl.elements["name"].value = subscription.name;
	formEl.elements["category"].value = subscription.category;
	formEl.elements["price"].value = subscription.price;
	formEl.elements["billing_cycle"].value = subscription.billing_cycle;
	formEl.elements["next_billing_date"].value = subscription.next_billing_date.slice(0, 10);
	showModal(modalOverlayEl);
}

function closeModal() {
	hideModal(modalOverlayEl);
	formEl.reset();
	delete formEl.dataset.editingId;
}

formEl.addEventListener("submit", async (event) => {
	event.preventDefault();
	const formData = new FormData(formEl);
	const payload = {
		name: formData.get("name"),
		category: formData.get("category"),
		price: Number(formData.get("price")),
		billing_cycle: formData.get("billing_cycle"),
		next_billing_date: formData.get("next_billing_date"),
	};

	const editingId = formEl.dataset.editingId;
	const url = editingId ? `/api/subscriptions/${editingId}` : "/api/subscriptions";
	const method = editingId ? "PUT" : "POST";

	const response = await fetch(url, {
		method,
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(payload),
	});
	const data = await response.json().catch(() => null);

	if (response.ok) {
		closeModal();
		showToast(editingId ? `${payload.name}を更新しました` : `${payload.name}を追加しました`, "success");
		await refresh();
	} else {
		showToast(data?.error ?? "処理に失敗しました", "error");
	}
});

// --- 削除確認モーダル ---

function openDeleteConfirm(id, name) {
	deleteTarget = { id, name };
	deleteTargetNameEl.textContent = name;
	showModal(deleteModalOverlayEl);
}

function closeDeleteModal() {
	hideModal(deleteModalOverlayEl);
	deleteTarget = null;
}

deleteCancelButtonEl.addEventListener("click", closeDeleteModal);

deleteConfirmButtonEl.addEventListener("click", async () => {
	if (!deleteTarget) return;
	const { id, name } = deleteTarget;
	const response = await fetch(`/api/subscriptions/${id}`, { method: "DELETE" });
	closeDeleteModal();
	if (response.ok) {
		showToast(`${name}を削除しました`, "success");
		await refresh();
	} else {
		showToast("削除に失敗しました", "error");
	}
});

// --- AI提案 ---

aiAdviceButtonEl.addEventListener("click", requestAiAdvice);
aiModalCloseButtonEl.addEventListener("click", () => hideModal(aiModalOverlayEl));

async function requestAiAdvice() {
	aiAdviceButtonEl.disabled = true;
	const originalText = aiAdviceButtonEl.textContent;
	aiAdviceButtonEl.textContent = "⏳ 相談中...";

	try {
		const response = await fetch("/api/ai/advice", { method: "POST" });
		const data = await response.json().catch(() => null);

		if (response.ok) {
			renderAiAdvice(data.advice, null);
			showModal(aiModalOverlayEl);
			showToast("AIからの提案を表示しました", "success");
		} else {
			const message = data?.error ?? "AIへの問い合わせに失敗しました";
			renderAiAdvice(null, message);
			showModal(aiModalOverlayEl);
			showToast(message, "error");
		}
	} catch (err) {
		const message = "AIへの問い合わせに失敗しました";
		renderAiAdvice(null, message);
		showModal(aiModalOverlayEl);
		showToast(message, "error");
	} finally {
		aiAdviceButtonEl.textContent = originalText;
		aiAdviceButtonEl.disabled = allSubscriptions.length === 0;
	}
}

function renderAiAdvice(adviceText, errorText) {
	aiModalContentEl.innerHTML = "";
	if (errorText) {
		const p = document.createElement("p");
		p.className = "text-red-600";
		p.textContent = errorText;
		aiModalContentEl.appendChild(p);
		return;
	}
	adviceText
		.split("\n")
		.filter((line) => line.trim() !== "")
		.forEach((line) => {
			const p = document.createElement("p");
			p.textContent = line;
			aiModalContentEl.appendChild(p);
		});
}

// --- 検索・カテゴリフィルター・並び替え ---

searchInputEl.addEventListener("input", (event) => {
	currentSearch = event.target.value;
	updateList();
});

categoryTabsEl.querySelectorAll("button[data-category]").forEach((tabButton) => {
	tabButton.addEventListener("click", () => {
		currentCategory = tabButton.dataset.category;
		categoryTabsEl.querySelectorAll("button[data-category]").forEach((btn) => {
			btn.classList.remove("bg-indigo-600", "text-white");
			btn.classList.add("bg-white", "text-slate-600");
		});
		tabButton.classList.remove("bg-white", "text-slate-600");
		tabButton.classList.add("bg-indigo-600", "text-white");
		updateList();
	});
});

sortSelectEl.addEventListener("change", (event) => {
	currentSort = event.target.value;
	updateList();
});

function applyFilters(subscriptions, keyword, category) {
	const normalizedKeyword = keyword.trim().toLowerCase();
	return subscriptions.filter((s) => {
		const matchesKeyword = normalizedKeyword === "" || s.name.toLowerCase().includes(normalizedKeyword);
		const matchesCategory = category === "all" || s.category === category;
		return matchesKeyword && matchesCategory;
	});
}

function sortSubscriptions(subscriptions, sortKey) {
	const sorted = [...subscriptions];
	switch (sortKey) {
		case "price-desc":
			sorted.sort((a, b) => b.annual_cost - a.annual_cost);
			break;
		case "price-asc":
			sorted.sort((a, b) => a.annual_cost - b.annual_cost);
			break;
		case "name":
			sorted.sort((a, b) => a.name.localeCompare(b.name, "ja"));
			break;
		case "renewal":
		default:
			sorted.sort((a, b) => a.days_until_renewal - b.days_until_renewal);
			break;
	}
	return sorted;
}

function updateList() {
	const filtered = applyFilters(allSubscriptions, currentSearch, currentCategory);
	const sorted = sortSubscriptions(filtered, currentSort);
	renderSubscriptions(sorted);
}

// --- 一覧描画 ---

// 更新日までの残り日数に応じたラベルとスタイルを返す
function renewalStatus(days) {
	if (days <= 3) return { label: `あと${days}日`, className: "bg-red-100 text-red-700" };
	if (days <= 7) return { label: `あと${days}日`, className: "bg-amber-100 text-amber-700" };
	return { label: `あと${days}日`, className: "bg-emerald-100 text-emerald-700" };
}

function formatYen(amount) {
	return `¥${amount.toLocaleString()}`;
}

function escapeHtml(str) {
	const div = document.createElement("div");
	div.textContent = str;
	return div.innerHTML;
}

function renderSubscriptions(subscriptions) {
	listEl.innerHTML = "";

	const hasVisible = subscriptions.length > 0;
	listEl.classList.toggle("hidden", !hasVisible);
	emptyMessageEl.classList.toggle("hidden", hasVisible);
	if (!hasVisible) {
		emptyMessageEl.textContent =
			allSubscriptions.length > 0
				? "😕 該当するサブスクが見つかりません"
				: "📭 登録されているサブスクはありません";
	}

	subscriptions.forEach((s) => {
		const status = renewalStatus(s.days_until_renewal);
		const categoryClass = CATEGORY_BADGE_CLASS[s.category] ?? CATEGORY_BADGE_CLASS["その他"];
		const cycleLabel = s.billing_cycle === "yearly" ? "年額" : "月額";
		const cycleClass = s.billing_cycle === "yearly" ? "bg-violet-100 text-violet-700" : "bg-sky-100 text-sky-700";
		const icon = guessIcon(s.name, s.category);

		const card = document.createElement("div");
		card.className = "bg-white rounded-xl shadow p-5 flex flex-col gap-3 hover:shadow-md transition-shadow";
		card.innerHTML = `
			<div class="flex items-start justify-between gap-2">
				<h3 class="font-bold text-slate-800">${icon} ${escapeHtml(s.name)}</h3>
				<div class="flex items-center gap-2 shrink-0">
					<button class="text-slate-400 hover:text-indigo-600" data-action="edit" aria-label="編集">✎</button>
					<button class="text-slate-400 hover:text-red-500" data-action="delete" aria-label="削除">✕</button>
				</div>
			</div>
			<div class="flex gap-2">
				<span class="text-xs px-2 py-1 rounded-full ${categoryClass}">${s.category}</span>
				<span class="text-xs px-2 py-1 rounded-full ${cycleClass}">${cycleLabel}</span>
			</div>
			<p class="text-2xl font-bold text-slate-800">
				${formatYen(s.price)}<span class="text-sm font-normal text-slate-400">/${s.billing_cycle === "yearly" ? "年" : "月"}</span>
			</p>
			<p class="text-xs text-slate-400">年間換算 ${formatYen(s.annual_cost)}</p>
			<div class="flex items-center justify-between pt-2">
				<span class="text-xs text-slate-500">${s.next_billing_date.slice(0, 10)}</span>
				<span class="text-xs px-2 py-1 rounded-full ${status.className}">${status.label}</span>
			</div>
		`;
		card.querySelector('[data-action="edit"]').addEventListener("click", () => openEditModal(s));
		card.querySelector('[data-action="delete"]').addEventListener("click", () => openDeleteConfirm(s.id, s.name));
		listEl.appendChild(card);
	});
}

// --- トースト通知 ---

function showToast(message, type = "success") {
	const toast = document.createElement("div");
	const typeClass = type === "error" ? "bg-red-600" : "bg-emerald-600";
	toast.className = `toast px-4 py-3 rounded-lg shadow text-sm font-medium text-white ${typeClass}`;
	toast.textContent = message;
	toastContainerEl.appendChild(toast);

	setTimeout(() => {
		toast.classList.add("toast-hide");
		setTimeout(() => toast.remove(), 300);
	}, 3000);
}

// --- データ取得 ---

async function loadSubscriptions() {
	const response = await fetch("/api/subscriptions");
	allSubscriptions = await response.json();
	aiAdviceButtonEl.disabled = allSubscriptions.length === 0;
	updateList();
}

async function loadSummary() {
	const response = await fetch("/api/summary");
	const summary = await response.json();
	totalAnnualCostEl.textContent = formatYen(summary.total_annual_cost);
	monthlyEquivalentCostEl.textContent = formatYen(summary.monthly_equivalent_cost);
	subscriptionCountEl.textContent = `${summary.subscription_count}件`;
}

async function refresh() {
	await Promise.all([loadSubscriptions(), loadSummary()]);
}

refresh();
