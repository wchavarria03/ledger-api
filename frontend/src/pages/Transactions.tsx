import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { Navbar } from '../components/Navbar'
import { ReportPanel } from '../components/reports/ReportPanel'
import { getAccount, getCategories, getTransactions, setTransactionCategories } from '../lib/api'
import type { Account, Category, Transaction, TxFilters, TxPage } from '../types'
import { displayName } from '../types'

const CURRENCY_SYMBOL: Record<string, string> = {
  CRC: '₡',
  USD: '$',
  EUR: '€',
}

const TYPE_STYLES: Record<string, string> = {
  expense: 'text-red-400',
  income: 'text-green-400',
  transfer: 'text-blue-400',
}

const TX_TYPES = [
  { value: '', label: 'All types' },
  { value: 'expense', label: 'Expense' },
  { value: 'income', label: 'Income' },
  { value: 'transfer', label: 'Transfer' },
]

function formatAmount(amount: string, currency: string): string {
  const symbol = CURRENCY_SYMBOL[currency] ?? currency
  const num = Number(amount)
  const formatted = Math.abs(num).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
  return num < 0 ? `-${symbol}${formatted}` : `${symbol}${formatted}`
}

function useDebounce<T>(value: T, delay: number): T {
  const [debounced, setDebounced] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setDebounced(value), delay)
    return () => clearTimeout(t)
  }, [value, delay])
  return debounced
}

export function Transactions() {
  const { id } = useParams<{ id: string }>()
  const [account, setAccount] = useState<Account | null>(null)
  const [page, setPage] = useState<TxPage | null>(null)
  const [categories, setCategories] = useState<Category[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Filter state
  const [search, setSearch] = useState('')
  const [type, setType] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [currentPage, setCurrentPage] = useState(1)

  const debouncedSearch = useDebounce(search, 300)

  // Load account + categories once
  useEffect(() => {
    if (!id) return
    Promise.all([getAccount(id), getCategories()])
      .then(([acc, cats]) => {
        setAccount(acc)
        setCategories(cats)
      })
      .catch((err: Error) => setError(err.message))
  }, [id])

  // Reload transactions whenever filters or page change
  useEffect(() => {
    if (!id) return
    setLoading(true)
    setError(null)
    const filters: TxFilters = { page: currentPage }
    if (debouncedSearch) filters.search = debouncedSearch
    if (type) filters.type = type
    if (from) filters.from = from
    if (to) filters.to = to
    getTransactions(id, filters)
      .then(setPage)
      .catch((err: Error) => setError(err.message))
      .finally(() => setLoading(false))
  }, [id, debouncedSearch, type, from, to, currentPage])

  // Reset to page 1 when filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [debouncedSearch, type, from, to])

  function handleCategoriesChanged(txId: string, newCats: Category[]) {
    setPage(prev => prev ? {
      ...prev,
      transactions: prev.transactions.map(tx =>
        tx.id === txId ? { ...tx, categories: newCats } : tx
      ),
    } : prev)
  }

  function clearFilters() {
    setSearch('')
    setType('')
    setFrom('')
    setTo('')
  }

  const hasFilters = search || type || from || to
  const currency = account?.currency ?? ''
  const symbol = CURRENCY_SYMBOL[currency] ?? currency

  return (
    <div className="min-h-screen">
      <Navbar />
      <main className="max-w-5xl mx-auto px-6 py-10 space-y-6">

        <div className="flex items-center gap-4">
          <Link to="/" className="text-gray-400 hover:text-white text-sm transition-colors">
            ← Back
          </Link>
          <h2 className="text-2xl font-semibold">Transactions</h2>
        </div>

        {account && (
          <div className="bg-gray-900 border border-gray-800 rounded-xl px-6 py-4 flex items-center justify-between">
            <div>
              <p className="font-semibold text-lg">{displayName(account)}</p>
              <p className="text-sm text-gray-400 uppercase mt-0.5">{account.bank_name}</p>
            </div>
            <div className="text-right">
              <span className="text-xs font-mono bg-gray-800 text-gray-300 px-3 py-1.5 rounded">
                {symbol} {currency}
              </span>
            </div>
          </div>
        )}

        {account && (
          <div className="space-y-3">
            <h2 className="text-xl font-semibold">Analytics</h2>
            <ReportPanel accountId={account.id} currency={account.currency} />
          </div>
        )}

        <div className="flex items-center justify-between">
          <h2 className="text-xl font-semibold">Transactions</h2>
          {page && (
            <p className="text-sm text-gray-400">
              {page.total} total
            </p>
          )}
        </div>

        {/* Filter bar */}
        <div className="flex flex-wrap gap-2 items-center">
          <input
            type="text"
            placeholder="Search description…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-gray-500 w-52"
          />
          <select
            value={type}
            onChange={e => setType(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-gray-500"
          >
            {TX_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
          <input
            type="date"
            value={from}
            onChange={e => setFrom(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-gray-500"
          />
          <span className="text-gray-500 text-sm">–</span>
          <input
            type="date"
            value={to}
            onChange={e => setTo(e.target.value)}
            className="bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm text-white focus:outline-none focus:border-gray-500"
          />
          {hasFilters && (
            <button
              onClick={clearFilters}
              className="text-xs text-gray-400 hover:text-white underline transition-colors"
            >
              Clear
            </button>
          )}
        </div>

        {error && (
          <div className="bg-red-900/20 border border-red-800 rounded-xl p-4 text-red-400 text-sm">{error}</div>
        )}

        {loading && (
          <div className="flex justify-center py-12">
            <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-blue-500" />
          </div>
        )}

        {!loading && !error && page && (
          <>
            <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-gray-800 text-gray-400 text-xs uppercase">
                    <th className="text-left px-4 py-3">Date</th>
                    <th className="text-left px-4 py-3">Description</th>
                    <th className="text-left px-4 py-3">Type</th>
                    <th className="text-left px-4 py-3">Category</th>
                    <th className="text-right px-4 py-3">Amount</th>
                    <th className="text-right px-4 py-3">Balance</th>
                  </tr>
                </thead>
                <tbody>
                  {page.transactions.map(tx => (
                    <tr key={tx.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                      <td className="px-4 py-3 text-gray-400 whitespace-nowrap">
                        {new Date(tx.date).toLocaleDateString()}
                      </td>
                      <td className="px-4 py-3">{tx.description}</td>
                      <td className={`px-4 py-3 capitalize ${TYPE_STYLES[tx.type] ?? 'text-gray-300'}`}>
                        {tx.type.replace('_', ' ')}
                      </td>
                      <td className="px-4 py-3">
                        <CategoryCell
                          transaction={tx}
                          allCategories={categories}
                          onChange={cats => handleCategoriesChanged(tx.id, cats)}
                        />
                      </td>
                      <td className={`px-4 py-3 text-right font-mono ${Number(tx.amount) < 0 ? 'text-red-400' : 'text-green-400'}`}>
                        {formatAmount(tx.amount, currency)}
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-gray-300">
                        {formatAmount(tx.balance, currency)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {page.transactions.length === 0 && (
                <p className="text-center text-gray-400 py-12">No transactions found</p>
              )}
            </div>

            {/* Pagination */}
            {page.total_pages > 1 && (
              <div className="flex items-center justify-between text-sm">
                <p className="text-gray-400">
                  Showing {((page.page - 1) * page.limit) + 1}–{Math.min(page.page * page.limit, page.total)} of {page.total}
                </p>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setCurrentPage(p => p - 1)}
                    disabled={page.page <= 1}
                    className="px-3 py-1.5 rounded-lg border border-gray-700 text-gray-300 hover:text-white hover:border-gray-500 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                  >
                    ← Prev
                  </button>
                  <span className="text-gray-400 px-2">
                    Page {page.page} of {page.total_pages}
                  </span>
                  <button
                    onClick={() => setCurrentPage(p => p + 1)}
                    disabled={page.page >= page.total_pages}
                    className="px-3 py-1.5 rounded-lg border border-gray-700 text-gray-300 hover:text-white hover:border-gray-500 transition-colors disabled:opacity-30 disabled:cursor-not-allowed"
                  >
                    Next →
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  )
}

// ── CategoryCell ──────────────────────────────────────────────────────────────

function CategoryCell({ transaction, allCategories, onChange }: {
  transaction: Transaction
  allCategories: Category[]
  onChange: (cats: Category[]) => void
}) {
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  const selectedIDs = new Set((transaction.categories ?? []).map(c => c.id))

  async function toggleCategory(cat: Category) {
    const next = new Set(selectedIDs)
    if (next.has(cat.id)) next.delete(cat.id)
    else next.add(cat.id)

    setSaving(true)
    try {
      await setTransactionCategories(transaction.id, [...next])
      const flat = flattenCategories(allCategories)
      onChange(flat.filter(c => next.has(c.id)))
    } finally {
      setSaving(false)
      setOpen(false)
    }
  }

  return (
    <div className="relative" ref={ref}>
      <div
        className="flex flex-wrap gap-1 cursor-pointer min-h-[24px]"
        onClick={() => setOpen(o => !o)}
      >
        {(transaction.categories ?? []).length === 0 ? (
          <span className="text-gray-600 text-xs hover:text-gray-400">+ add</span>
        ) : (
          (transaction.categories ?? []).map(cat => (
            <CategoryBadge key={cat.id} category={cat} />
          ))
        )}
      </div>

      {open && (
        <div className="absolute left-0 top-full mt-1 z-20 bg-gray-900 border border-gray-700 rounded-xl shadow-xl w-56 py-1 max-h-72 overflow-y-auto">
          {saving && <p className="text-xs text-gray-400 px-3 py-2">Saving…</p>}
          {!saving && allCategories.map(parent => (
            <div key={parent.id}>
              <button
                onClick={() => toggleCategory(parent)}
                className={`w-full text-left px-3 py-1.5 text-sm hover:bg-gray-800 flex items-center gap-2 ${selectedIDs.has(parent.id) ? 'text-white' : 'text-gray-300'}`}
              >
                <span className="w-2 h-2 rounded-full flex-shrink-0" style={{ background: parent.color ?? '#6b7280' }} />
                {parent.name}
                {selectedIDs.has(parent.id) && <span className="ml-auto text-blue-400 text-xs">✓</span>}
              </button>
              {(parent.children ?? []).map(child => (
                <button
                  key={child.id}
                  onClick={() => toggleCategory(child)}
                  className={`w-full text-left pl-7 pr-3 py-1.5 text-xs hover:bg-gray-800 flex items-center gap-2 ${selectedIDs.has(child.id) ? 'text-white' : 'text-gray-400'}`}
                >
                  <span className="w-1.5 h-1.5 rounded-full flex-shrink-0" style={{ background: child.color ?? '#6b7280' }} />
                  {child.name}
                  {selectedIDs.has(child.id) && <span className="ml-auto text-blue-400 text-xs">✓</span>}
                </button>
              ))}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function CategoryBadge({ category }: { category: Category }) {
  return (
    <span
      className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full font-medium"
      style={{ background: (category.color ?? '#6b7280') + '33', color: category.color ?? '#9ca3af', border: `1px solid ${category.color ?? '#6b7280'}44` }}
    >
      {category.name}
    </span>
  )
}

function flattenCategories(tree: Category[]): Category[] {
  const result: Category[] = []
  for (const cat of tree) {
    result.push(cat)
    if (cat.children) result.push(...cat.children)
  }
  return result
}
