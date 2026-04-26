# AppAuthGuard.test.tsx flaky test 根本原因調査・修正レポート

## 再現実験の結果（スタック頻度）

5回連続実行、全て 30秒 timeout で `Terminated` → **スタック率 100%**

```
timeout 30 npm test -- AppAuthGuard --run 2>&1 | tail -5
Terminated  ← run 1
Terminated  ← run 2
Terminated  ← run 3
Terminated  ← run 4
Terminated  ← run 5
```

## 根本原因特定プロセス（仮説検証）

### 仮説検証一覧

| 仮説 | 検証結果 |
|------|---------|
| (a) fake timer と real timer の混在 | テストファイルに `vi.useFakeTimers` なし → 対象外 |
| (b) 非同期 race condition (waitFor 不足) | テストは同期のみ、waitFor 使用なし → 対象外 |
| (c) Promise 未解決 | mock は全て resolved → 対象外 |
| (d) React state 更新の競合 (act 漏れ) | `render()` は RTL が `act()` でラップ → 対象外 |
| (e) sessionStorage/localStorage 汚染 | AppAuthGuard はストレージ直接使用なし → 対象外 |
| (f) Context Provider の async 初期化前アサート | `useAppAuth` は vi.mock で完全モック化 → 対象外 |
| (g) fetch mock / MSW 未応答リクエスト | fetch 呼び出しなし → 対象外 |
| **無限リダイレクトループ** | **コード解析で確定 → 根本原因** |

### 根本原因の特定（コード解析）

**確定した根本原因: `AppAuthGuard` が `<Routes>` 外に配置されており、リダイレクト後も常にマウントされたまま無限に新しいリダイレクト URL を計算し続ける。**

React Router v6 の `<Navigate>` は `useEffect` (依存配列なし) を使って `navigate()` を呼ぶ:
```
useEffect(() => { navigate(to, { replace, state }); });
```

修正前のコード:
```tsx
<MemoryRouter initialEntries={[path]}>
  <LocationCapture />   // ← 常にマウント
  <AppAuthGuard>        // ← 常にマウント
    <div>...</div>
  </AppAuthGuard>
</MemoryRouter>
```

実行フロー（無限ループ）:
1. `/dashboard?tab=security` でレンダリング、user=null
2. `AppAuthGuard` → `<Navigate to="/login?redirect=%2Fdashboard%3Ftab%3Dsecurity">`
3. `Navigate.useEffect` → navigate → location が `/login?redirect=...` に変更
4. `AppAuthGuard` が新しい location で再レンダリング
5. `AppAuthGuard` → `<Navigate to="/login?redirect=%2Flogin%3Fredirect%3D...">`（新しい URL!）
6. `Navigate.useEffect` → navigate → location がさらに変更
7. ループ継続...

`act()` はキューが空になるまでフラッシュし続けるが、常に新しい navigation がキューに追加されるため無限ループになる。

## 修正内容

**ファイル**: `apps/frontend/src/components/AppAuthGuard.test.tsx`

**変更の本質**: `renderGuard` 内で `<Routes>` を使って適切なルートツリーを構築し、`/login` へのナビゲーション後に `AppAuthGuard` がアンマウントされるようにする。

```tsx
// Before
function renderGuard(path = '/') {
  capturedPath = path;
  return render(
    <MemoryRouter initialEntries={[path]}>
      <LocationCapture />
      <AppAuthGuard>
        <div data-testid="children">protected content</div>
      </AppAuthGuard>
    </MemoryRouter>,
  );
}

// After
// <Routes> is required: without it, AppAuthGuard stays mounted after Navigate fires and infinitely recomputes redirect URLs.
function renderGuard(path = '/') {
  capturedPath = path;
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<LocationCapture />} />
        <Route
          path="*"
          element={
            <AppAuthGuard>
              <div data-testid="children">protected content</div>
            </AppAuthGuard>
          }
        />
      </Routes>
    </MemoryRouter>,
  );
}
```

追加変更: `LocationCapture` の `useEffect` は維持（lint ルール `react-hooks/globals` に準拠）。

## 修正後の連続実行結果（10回全PASS）

```
Duration  1.50s  --- run 1 ---
Duration  1.55s  --- run 2 ---
Duration  1.61s  --- run 3 ---
Duration  1.58s  --- run 4 ---
Duration  1.59s  --- run 5 ---
Duration  1.50s  --- run 6 ---
Duration  1.51s  --- run 7 ---
Duration  1.49s  --- run 8 ---
Duration  1.63s  --- run 9 ---
Duration  1.58s  --- run 10 ---
```

10/10 PASS、各実行 ~1.5秒で完了。

## mise run check 結果

```
Test Files  19 passed (19)
      Tests  112 passed (112)
   Start at  03:12:35
   Duration  18.35s
```

SKIP=0, FAIL=0, lint エラーなし。
