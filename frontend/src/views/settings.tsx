import { useTranslation } from 'react-i18next'
import { useLLMConfigs, useActiveLLMConfig, useSaveLLMConfig, useTestLLMConnection, useSetSetting } from '@/hooks/wails'
import * as App from '../../wailsjs/go/main/App'
import { useSettingsStore } from '@/stores/settings'
import { Save, Loader2, CheckCircle, AlertCircle } from 'lucide-react'
import { useState, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'

export function Settings() {
  const { t, i18n } = useTranslation()
  const { data: configs } = useLLMConfigs()
  const { data: activeConfig } = useActiveLLMConfig()
  const saveConfig = useSaveLLMConfig()
  const testConnection = useTestLLMConnection()
  const setSetting = useSetSetting()
  const { theme, language, contentPreview, setTheme, setLanguage, setContentPreview } = useSettingsStore()
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
              <div key={cfg.id} className="flex items-center gap-3 p-3 rounded-lg border border-border">
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

      {/* Scan Preferences */}
      <section className="space-y-4">
        <h2 className="text-lg font-semibold">{t('settings.scan.title')}</h2>
        <div className="flex items-center gap-3">
          <Checkbox
            id="content-preview"
            checked={contentPreview}
            onCheckedChange={(v) => {
              const checked = v === true
              setContentPreview(checked)
              setSetting.mutate({ key: 'content_preview', value: String(checked) })
            }}
          />
          <Label htmlFor="content-preview">{t('settings.scan.contentPreview')}</Label>
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

      {version && (
        <p className="text-xs text-muted-foreground text-center pt-4">DFCleaner {version}</p>
      )}
    </div>
  )
}
