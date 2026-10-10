import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ScrollView {
    id: root
    required property var shell
    clip: true
    contentWidth: availableWidth
    property var providers: [
        {
            id: "outlook",
            label: "Outlook"
        },
        {
            id: "icloud",
            label: "iCloud"
        },
        {
            id: "moemail",
            label: "MoeMail"
        },
        {
            id: "cloudmail",
            label: "Cloud-Mail"
        },
        {
            id: "mailnest",
            label: "MailNest"
        }
    ]
    property string provider: providers[providerChoice.currentIndex]?.id || "outlook"
    property var configs: backend.providerConfigurations.filter(c => c.provider === provider)
    property var proxyChoices: backend.proxies.filter(p => p.enabled)
    property bool pooled: provider === "outlook" || provider === "icloud"
    property bool running: !!backend.batchStatus.running
    ColumnLayout {
        width: root.availableWidth
        spacing: 16
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: form.implicitHeight + 48
            dark: root.shell.dark
            ColumnLayout {
                id: form
                anchors {
                    fill: parent
                    margins: 24
                }
                spacing: 16
                Text {
                    text: root.shell.t("新建注册任务", "New registration batch", "登録タスクを作成")
                    color: root.shell.ink
                    font {
                        pixelSize: 18
                        weight: Font.DemiBold
                    }
                }
                Text {
                    text: root.shell.t("选择邮箱来源与连接方式，成功结果会写入输出目录。", "Choose your mail source and connection. Successful results go to your output folder.", "メールと接続方法を選択します。成功結果は出力フォルダーに保存します。")
                    color: root.shell.muted
                    font.pixelSize: 12
                    wrapMode: Text.Wrap
                    Layout.fillWidth: true
                }
                GridLayout {
                    Layout.fillWidth: true
                    columns: 2
                    columnSpacing: 20
                    rowSpacing: 12
                    enabled: !root.running
                    ColumnLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.shell.t("邮箱来源", "Mail source", "メールの種類")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassComboBox {
                            id: providerChoice
                            Layout.fillWidth: true
                            dark: root.shell.dark
                            model: root.providers.map(p => p.label)
                            Accessible.name: root.shell.t("邮箱来源", "Mail source", "メールの種類")
                            onCurrentIndexChanged: configChoice.currentIndex = 0
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.shell.t("网络连接", "Connection", "接続方法")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassComboBox {
                            id: proxyChoice
                            Layout.fillWidth: true
                            dark: root.shell.dark
                            model: [root.shell.t("直连", "Direct", "直接接続"), root.shell.t("按权重使用代理池", "Weighted proxy pool", "プロキシプールを使用")].concat(root.proxyChoices.map(p => p.name || p.url))
                            Accessible.name: root.shell.t("网络连接", "Connection", "接続方法")
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: !root.pooled
                        Text {
                            text: root.shell.t("邮箱服务", "Mail service", "メールサービス")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassComboBox {
                            id: configChoice
                            Layout.fillWidth: true
                            dark: root.shell.dark
                            model: [root.shell.t("全部已保存服务", "All saved services", "保存済みサービスすべて")].concat(root.configs.map(c => c.name || "MailNest"))
                            Accessible.name: root.shell.t("邮箱服务", "Mail service", "メールサービス")
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: !root.pooled && root.provider !== "mailnest"
                        Text {
                            text: root.shell.t("域名（可选，逗号分隔）", "Domains (optional, comma separated)", "ドメイン（任意、カンマ区切り）")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassTextField {
                            id: domains
                            Layout.fillWidth: true
                            dark: root.shell.dark
                            placeholderText: "example.com, example.net"
                            Accessible.name: root.shell.t("域名", "Domains", "ドメイン")
                        }
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    visible: !root.pooled && root.provider !== "mailnest"
                    enabled: !root.running
                    Text {
                        text: root.shell.t("域名分配", "Domain selection", "ドメイン選択")
                        color: root.shell.muted
                        font.pixelSize: 12
                        Layout.fillWidth: true
                    }
                    GlassComboBox {
                        id: domainMode
                        dark: root.shell.dark
                        model: [root.shell.t("轮询", "Round robin", "順番に選択"), root.shell.t("随机", "Random", "ランダム")]
                        Accessible.name: root.shell.t("域名分配", "Domain selection", "ドメイン選択")
                    }
                }
                GridLayout {
                    Layout.fillWidth: true
                    enabled: !root.running
                    columns: 3
                    columnSpacing: 16
                    ColumnLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.shell.t("数量", "Count", "件数")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassSpinBox {
                            id: count
                            Layout.fillWidth: true
                            from: 1
                            to: 10000
                            value: 1
                            editable: true
                            Accessible.name: root.shell.t("注册数量", "Registration count", "登録件数")
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.shell.t("并发", "Concurrency", "並列数")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassSpinBox {
                            id: concurrency
                            Layout.fillWidth: true
                            from: 1
                            to: 64
                            value: 1
                            editable: true
                            Accessible.name: root.shell.t("并发数量", "Concurrency", "並列数")
                        }
                    }
                    ColumnLayout {
                        Layout.fillWidth: true
                        Text {
                            text: root.shell.t("间隔（秒）", "Delay (s)", "間隔（秒）")
                            color: root.shell.muted
                            font.pixelSize: 12
                        }
                        GlassSpinBox {
                            id: delay
                            Layout.fillWidth: true
                            from: 0
                            to: 86400
                            value: 1
                            editable: true
                            Accessible.name: root.shell.t("任务间隔", "Task delay", "タスクの間隔")
                        }
                    }
                }
                RowLayout {
                    Layout.fillWidth: true
                    Text {
                        Layout.fillWidth: true
                        color: root.shell.muted
                        font.pixelSize: 12
                        wrapMode: Text.Wrap
                        text: root.pooled ? root.shell.t("可用邮箱：", "Available mailboxes: ", "利用可能なメール：") + backend.mailboxes.filter(a => a.provider === root.provider && !a.registered).length : root.shell.t("每个任务分配一个新邮箱。", "One new mailbox per task.", "タスクごとに新しいメールを割り当てます。")
                    }
                    GlassButton {
                        visible: !root.running
                        text: root.shell.t("开始任务", "Start batch", "タスクを開始")
                        primary: true
                        dark: root.shell.dark
                        reduceMotion: root.shell.reduceMotion
                        enabled: !backend.busy
                        onClicked: backend.startBatch({
                            provider: root.provider,
                            count: count.value,
                            concurrency: concurrency.value,
                            delaySeconds: delay.value,
                            configurationNames: configChoice.currentIndex > 0 ? [root.configs[configChoice.currentIndex - 1].name] : [],
                            domains: domains.text.split(/[,，\s]+/).filter(d => d.length > 0),
                            randomDomains: domainMode.currentIndex === 1,
                            proxyMode: proxyChoice.currentIndex === 0 ? "direct" : proxyChoice.currentIndex === 1 ? "pool" : "selected",
                            proxyId: proxyChoice.currentIndex > 1 ? root.proxyChoices[proxyChoice.currentIndex - 2].id : ""
                        })
                    }
                    GlassButton {
                        visible: root.running
                        text: backend.batchStatus.stopping ? root.shell.t("正在停止…", "Stopping…", "停止中…") : root.shell.t("停止任务", "Stop batch", "タスクを停止")
                        danger: true
                        dark: root.shell.dark
                        enabled: !backend.batchStatus.stopping
                        onClicked: backend.stopBatch()
                    }
                }
            }
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: 108
            dark: root.shell.dark
            RowLayout {
                anchors {
                    fill: parent
                    margins: 24
                }
                ColumnLayout {
                    Layout.fillWidth: true
                    Text {
                        text: root.shell.t("任务进度", "Batch progress", "タスクの進捗")
                        color: root.shell.ink
                        font {
                            pixelSize: 15
                            weight: Font.DemiBold
                        }
                    }
                    Text {
                        text: (backend.batchStatus.completed || 0) + " / " + (backend.batchStatus.total || 0) + "  ·  " + root.shell.t("成功", "Succeeded", "成功") + " " + (backend.batchStatus.success || 0) + "  ·  " + root.shell.t("失败", "Failed", "失敗") + " " + (backend.batchStatus.failed || 0) + "  ·  " + root.shell.t("取消", "Cancelled", "キャンセル") + " " + (backend.batchStatus.cancelled || 0)
                        color: root.shell.muted
                        font.pixelSize: 12
                    }
                    ProgressBar {
                        Layout.fillWidth: true
                        from: 0
                        to: Math.max(1, backend.batchStatus.total || 0)
                        value: backend.batchStatus.completed || 0
                    }
                }
                Text {
                    text: Math.floor(backend.batchStatus.elapsed || 0) + "s"
                    color: root.shell.muted
                    font.pixelSize: 13
                }
            }
        }
        Text {
            visible: !!backend.batchStatus.riskStopped
            text: root.shell.t("检测到风控响应，任务已停止。", "A risk response stopped this batch.", "リスク応答によりタスクが停止しました。")
            color: "#d76c79"
            font.pixelSize: 13
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: Math.max(160, Math.min(460, backend.tasks.length * 64 + 40))
            dark: root.shell.dark
            ListView {
                anchors {
                    fill: parent
                    margins: 20
                }
                clip: true
                model: backend.tasks
                spacing: 8
                ScrollBar.vertical: ScrollBar {}
                delegate: Rectangle {
                    required property var modelData
                    width: ListView.view.width
                    height: 56
                    radius: 14
                    color: root.shell.dark ? "#2c364a" : "#eef2fb"
                    RowLayout {
                        anchors {
                            fill: parent
                            margins: 14
                        }
                        Text {
                            text: "#" + modelData.index
                            color: root.shell.muted
                            font.pixelSize: 12
                            Layout.preferredWidth: 48
                        }
                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 2
                            Text {
                                text: modelData.email || (root.shell.uiLanguage && backend.stepLabel(modelData.step || "")) || root.shell.statusLabel(modelData.status)
                                color: root.shell.ink
                                font.pixelSize: 12
                                elide: Text.ElideMiddle
                                Layout.fillWidth: true
                            }
                            Text {
                                visible: !!modelData.errorCode || !!modelData.warning
                                text: modelData.errorCode || (modelData.warning === "MAILBOX_STATUS_NOT_SAVED" ? root.shell.t("邮箱状态未保存，任务已停止", "Mailbox status was not saved; batch stopped", "メール状態を保存できずタスクを停止しました") : root.shell.t("结果已保存至恢复目录", "Results saved to recovery folder", "結果は復旧フォルダーに保存されました"))
                                color: root.shell.muted
                                font.pixelSize: 10
                            }
                        }
                        Text {
                            text: root.shell.statusLabel(modelData.status)
                            color: modelData.status === "success" ? "#54b195" : modelData.status === "failed" ? "#d76c79" : root.shell.muted
                            font.pixelSize: 12
                        }
                    }
                }
                Text {
                    anchors.centerIn: parent
                    visible: backend.tasks.length === 0
                    text: root.shell.t("任务记录将在这里显示。", "Your batch activity will appear here.", "タスクの記録がここに表示されます。")
                    color: root.shell.muted
                    font.pixelSize: 13
                }
            }
        }
    }
}
