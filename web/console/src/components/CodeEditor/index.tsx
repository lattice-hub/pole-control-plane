import React from 'react';
import { Editor as MonacoEditor, loader } from '@monaco-editor/react'
import type { OnChange, OnMount } from '@monaco-editor/react'
import { Icon, LoadingIcon } from 'components/Fluent/icons';
import type { editor } from 'monaco-editor';
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api';
import 'monaco-editor/esm/vs/editor/editor.all';
import 'monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution';
import 'monaco-editor/esm/vs/basic-languages/ini/ini.contribution';
import 'monaco-editor/esm/vs/basic-languages/xml/xml.contribution';
import 'monaco-editor/esm/vs/language/json/monaco.contribution';
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker';
import JsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker';
(globalThis as typeof globalThis & { MonacoEnvironment: { getWorker: (_id: string, label: string) => Worker } }).MonacoEnvironment = {
  getWorker: (_id, label) => label === 'json' ? new JsonWorker() : new EditorWorker(),
};
loader.config({ monaco });
import Style from './index.module.less';
import classNames from 'classnames';
import { useAppSelector } from 'modules/store';
import { selectGlobal } from 'modules/global';
import { ETheme } from 'types/index.d';

export interface ICodeEditorProps {
    readonly?: boolean;
    modelPath?: string;
    ariaLabel?: string;
    validate?: boolean;
    allowFullScreen?: boolean;
    value?: string;
    language?: string;
    theme?: string;
    height?: string | number;
    onChange?: (value: string | undefined, event: any) => void;
    onMount?: (editor: any, monaco: any) => void;
}

export function toHighlightLanguage(format?: string) {
    if (!format) {
      return "text"
    }
    if (format === "properties") {
      return 'ini'
    }
    if (format === 'yml') {
      return 'yaml'
    }
    return format
  }

const CodeEditor: React.FC<ICodeEditorProps> = props => {
    const { theme } = useAppSelector(selectGlobal);
    // 默认不开启全屏
    const [isFullScreen, setFullScreen] = React.useState(false)
    const editorRef = React.useRef<editor.IStandaloneCodeEditor>(null!);
    const [rect, setRect] = React.useState({ width: 0, height: 0 })

    const handleEditorChange: OnChange = (value, event) => {
        if (props.onChange) {
            props.onChange(value, event);
        }
    }

    const handleEditorDidMount: OnMount = (editor: editor.IStandaloneCodeEditor, monaco) => {
        editorRef.current = editor;
        props.onMount?.(editor, monaco);
    }

    const handleFullScreen = () => {
        setFullScreen(!isFullScreen)
        if (editorRef.current) {
            if (isFullScreen) {
                editorRef.current.layout({
                    height: rect.height,
                    width: rect.width,
                })
            } else {
                const originRect: DOMRect = editorRef.current.getContainerDomNode().getBoundingClientRect();
                if (originRect) {
                    setRect({ width: originRect.width, height: originRect.height });
                    setFullScreen(true);
                    setTimeout(() => {
                        editorRef.current?.layout({
                            height: document.body.clientHeight,
                            width: document.body.clientWidth,
                        });
                    }, 0);
                }
            }
        }
    }

    return (
        <section
            className={classNames({
                [Style.monacoSection]: true,
                [Style.monacoSectionFull]: isFullScreen,
            })}
            style={
                isFullScreen
                    ? {
                        position: 'fixed',
                        top: 0,
                        left: 0,
                        right: 0,
                        bottom: 0,
                        width: '100vw',
                        height: '100vh',
                        zIndex: 2000,
                    }
                    : {
                        height: props.height || 'calc(100vh - 390px)',
                        width: "100%",
                        position: 'relative'
                    }
            }
        >
            {props.allowFullScreen && (
                <Icon
                    name={!isFullScreen ? 'fullscreen-1' : 'fullscreen-exit-1'}
                    style={{
                        position: 'absolute',
                        right: 100,
                        top: 0,
                        zIndex: 999,
                    }}
                    onClick={handleFullScreen}
                />
            )}
            <MonacoEditor
                path={props.modelPath}
                saveViewState
                loading={<LoadingIcon />}
                theme={props.theme || (theme === ETheme.dark ? 'vs-dark' : 'vs')}
                language={toHighlightLanguage(props.language)}
                value={props.value}
                onMount={handleEditorDidMount}
                onChange={handleEditorChange}
                options={{
                    // 控制是否只读
                    ariaLabel: props.ariaLabel || '配置正文',
                    renderValidationDecorations: props.validate === false ? 'off' : 'on',
                    readOnly: props.readonly,
                    minimap: { enabled: false },
                    scrollBeyondLastLine: false,
                    wordWrap: 'on',
                    fontSize: 13,
                    lineHeight: 22,
                    tabSize: 2,
                    // 控制是否显示行号
                    lineNumbers: 'on',
                    //
                    automaticLayout: true,
                }}
            />
        </section>
    )
}

export default React.memo(CodeEditor);
