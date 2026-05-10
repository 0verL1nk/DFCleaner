import { useTranslation } from 'react-i18next'
import { useLLMConfigs, useActiveLLMConfig, useSaveLLMConfig, useTestLLMConnection, useSetSetting, useCheckForUpdate, usePerformUpdate } from '@/hooks/wails'
import * as App from '../../wailsjs/go/main/App'
import { useSettingsStore } from '@/stores/settings'
import { Save, Loader2, CheckCircle, AlertCircle, RefreshCw, Download } from 'lucide-react'
import { useState, useEffect } from 'react'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'

export function Settings() {
  const { t, i18n } = useTranslation()
  const { data: configs } = useLLMConfigs()
  const { data: activeConfig } = useActiveLLMConfig()
  const saveConfig = useSaveLLMConfig()
  const testConnection = useTestLLMConnection()
  const setSetting = useSetSetting()
  const { theme, language, setTheme, setLanguage } = useSettingsStore()
  const [testResult, setTestResult] = useState<{ success: boolean; msg: string } | null>(null)
  const [version, setVersion] = useState('')

  const [provider, setProvider] = useState('openai')
  const [endpoint, setEndpoint] = useState('https://api.openai.com/v1')
  const [apiKey, setApiKey] = useState('')
  const [modelName, setModelName] = useState('gpt-4o')

  useEffect(() => {
    if (activeConfig) {
      setProvider(activeConfig.provider || 'openai')
      setEndpoint(activeConfig.endpoint || '')
      setApiKey(activeConfig.apiKey || '')
      setModelName(activeConfig.modelName || '')
    }
  }, [activeConfig])

  useEffect(() => {
    App.GetVersion().then((v) => setVersion(v))
  }, [])

  function getFormConfig() {
    return { provider, endpoint, apiKey, modelName, isActive: true }
  }

  function handleSave() {
    saveConfig.mutate(getFormConfig())
  }

  async function handleTest() {
    setTestResult(null)
    try {
      const result = await testConnection.mutateAsync(getFormConfig()) as any
      if (result.success) {
        setTestResult({ success: true, msg: result.noFunctionCalling ? t('settings.llm.noFc') : t('settings.llm.connected') })
      } else {
        setTestResult({ success: false, msg: result.error ?? 'Unknown error' })
      }
    } catch (err: any) {
      setTestResult({ success: false, msg: err.message || 'Connection failed' })
    }
  }

  function handleThemeChange(v: string) {
    setTheme(v as 'light' | 'dark' | 'system')
    setSetting.mutate({ key: 'theme', value: v })
  }

  function handleLanguageChange(v: string) {
    setLanguage(v as 'en' | 'zh')
    setSetting.mutate({ key: 'language', value: v })
    i18n.changeLanguage(v)
  }

  return (
    <div className="p-6 max-w-3xl mx-auto space-y-8">
      <h1 className="text-2xl font-bold">{t('nav.settings')}</h1>

      {/* LLM Config */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">{t('settings.llm.title')}</h2>

        {configs && configs.length > 0 && (
          <div className="space-y-2">
            {configs.map((cfg: any) => (
              <div key={`${cfg.provider}-${cfg.modelName}`} className="flex items-center gap-3 p-3 rounded-lg border border-border">
                <span className="text-sm font-medium">{cfg.provider}</span>
                <span className="text-sm text-muted-foreground">{cfg.modelName}</span>
                <span className="text-xs text-muted-foreground flex-1 truncate">{cfg.endpoint}</span>
                {cfg.isActive && (
                  <Badge variant="secondary" className="text-safe">
                    <CheckCircle className="w-3 h-3 mr-1" /> {t('settings.llm.active')}
                  </Badge>
                )}
              </div>
            ))}
          </div>
        )}

        <Separator />

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{t('settings.llm.provider')}</Label>
              <Select value={provider} onValueChange={setProvider}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="openai">OpenAI</SelectItem>
                  <SelectItem value="deepseek">DeepSeek</SelectItem>
                  <SelectItem value="openrouter">OpenRouter</SelectItem>
                  <SelectItem value="qianfan">Qianfan</SelectItem>
                  <SelectItem value="claude">Claude</SelectItem>
                  <SelectItem value="ollama">Ollama</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>{t('settings.llm.model')}</Label>
              <Input value={modelName} onChange={(e) => setModelName(e.target.value)} placeholder="gpt-4o" />
            </div>
          </div>
          <div className="space-y-2">
            <Label>{t('settings.llm.endpoint')}</Label>
            <Input value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://api.openai.com/v1" />
          </div>
          <div className="space-y-2">
            <Label>{t('settings.llm.apiKey')}</Label>
            <Input value={apiKey} onChange={(e) => setApiKey(e.target.value)} type="password" />
          </div>
          <div className="flex gap-2">
            <Button onClick={handleSave} disabled={saveConfig.isPending || !endpoint || !apiKey || !modelName}>
              {saveConfig.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
              {t('common.save')}
            </Button>
            <Button variant="secondary" onClick={handleTest} disabled={testConnection.isPending || !endpoint || !apiKey}>
              {testConnection.isPending && <Loader2 className="w-4 h-4 animate-spin mr-2" />}
              {t('settings.llm.test')}
            </Button>
          </div>
          {testResult && (
            <div className={`flex items-center gap-2 text-sm ${testResult.success ? 'text-safe' : 'text-destructive'}`}>
              {testResult.success ? <CheckCircle className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
              {testResult.msg}
            </div>
          )}
        </div>
      </section>

      <Separator />

      {/* Appearance */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">{t('settings.appearance.title')}</h2>
        <div className="grid grid-cols-2 gap-4">
          <div className="space-y-2">
            <Label>{t('settings.appearance.theme')}</Label>
            <Select value={theme} onValueChange={handleThemeChange}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="system">{t('settings.appearance.system')}</SelectItem>
                <SelectItem value="light">{t('settings.appearance.light')}</SelectItem>
                <SelectItem value="dark">{t('settings.appearance.dark')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>{t('settings.appearance.language')}</Label>
            <Select value={language} onValueChange={handleLanguageChange}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="en">English</SelectItem>
                <SelectItem value="zh">中文</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
      </section>

      {/* About */}
      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm font-medium">DFCleaner {version}</p>
            <UpdateChecker />
          </div>
        </div>
      </section>
    </div>
  )
}

function UpdateChecker() {
  const { t } = useTranslation()
  const checkUpdate = useCheckForUpdate()
  const performUpdate = usePerformUpdate()
  const [updateInfo, setUpdateInfo] = useState<{ hasUpdate: boolean; latestVer: string; downloadUrl: string } | null>(null)
  const [updateProgress, setUpdateProgress] = useState<{ phase: string; progress: number; message: string } | null>(null)

  useEffect(() => {
    const off = EventsOn('update:progress', (data: any) => {
      setUpdateProgress({ phase: data.phase, progress: data.progress, message: data.message })
    })
    return () => { off() }
  }, [])

  async function handleCheck() {
    setUpdateInfo(null)
    setUpdateProgress(null)
    try {
      const result = await checkUpdate.mutateAsync() as any
      setUpdateInfo({ hasUpdate: result.hasUpdate, latestVer: result.latestVer, downloadUrl: result.downloadUrl })
    } catch {
      setUpdateInfo(null)
    }
  }

  function handleUpdate() {
    if (!updateInfo) return
    performUpdate.mutate({
      hasUpdate: updateInfo.hasUpdate,
      latestVer: updateInfo.latestVer,
      downloadUrl: updateInfo.downloadUrl,
    })
  }

  const isUpdating = performUpdate.isPending || updateProgress !== null

  return (
    <div className="mt-1 space-y-2">
      <div className="flex items-center gap-2">
        <Button size="sm" variant="ghost" onClick={handleCheck} disabled={checkUpdate.isPending || isUpdating}>
          {checkUpdate.isPending ? <Loader2 className="w-3 h-3 animate-spin mr-1" /> : <RefreshCw className="w-3 h-3 mr-1" />}
          {t('settings.about.checkUpdate')}
        </Button>
        {updateInfo && !updateInfo.hasUpdate && (
          <span className="text-xs text-muted-foreground">{t('settings.about.upToDate')}</span>
        )}
      </div>

      {updateInfo && updateInfo.hasUpdate && !isUpdating && (
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">
            {t('settings.about.updateAvailable', { version: updateInfo.latestVer })}
          </span>
          <Button size="sm" onClick={handleUpdate}>
            <Download className="w-3 h-3 mr-1" />
            {t('settings.about.updateAndRestart')}
          </Button>
        </div>
      )}

      {isUpdating && updateProgress && (
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Loader2 className="w-3 h-3 animate-spin" />
            <span className="text-xs text-muted-foreground">{updateProgress.message}</span>
          </div>
          <div className="w-48 h-1.5 bg-secondary rounded-full overflow-hidden">
            <div
              className="h-full bg-primary rounded-full transition-all duration-300"
              style={{ width: `${updateProgress.progress}%` }}
            />
          </div>
        </div>
      )}

      {performUpdate.isError && (
        <span className="text-xs text-destructive">{t('settings.about.updateFailed')}</span>
      )}
    </div>
  )
}