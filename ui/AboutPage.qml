import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

ScrollView {
    id: root
    required property var shell
    contentWidth: availableWidth
    clip: true
    ColumnLayout {
        width: root.availableWidth
        spacing: 18
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: 300
            dark: root.shell.dark
            ColumnLayout {
                anchors { fill: parent; margins: 28 }
                spacing: 14
                RowLayout {
                    spacing: 18
                    Image { source: "assets/kirox-light.svg"; Layout.preferredWidth: 68; Layout.preferredHeight: 68 }
                    ColumnLayout {
                        Text { text: "KiroX"; color: root.shell.ink; font { pixelSize: 25; weight: Font.Bold } }
                        Text { text: "2.0.0"; color: root.shell.muted; font.pixelSize: 13 }
                    }
                }
                Text { text: root.shell.t("一个清晰、专注的 Kiro 注册工作空间。", "A focused workspace for Kiro registration.", "Kiro 登録のための、集中できるワークスペース。"); color: root.shell.muted; font.pixelSize: 13 }
                RowLayout {
                    GlassButton { text: "GitHub"; dark: root.shell.dark; onClicked: backend.openUrl("https://github.com/huey1in/KiroX") }
                    GlassButton { text: root.shell.t("加入交流群", "Community", "コミュニティ"); dark: root.shell.dark; onClicked: backend.openUrl("https://qm.qq.com/q/RXMTXUlc4w") }
                    GlassButton { text: root.shell.t("检查更新", "Check updates", "更新を確認"); dark: root.shell.dark; enabled: !backend.updateChecking; onClicked: backend.checkUpdates() }
                }
                Text { Layout.fillWidth: true; wrapMode: Text.Wrap; color: root.shell.muted; font.pixelSize: 12; text: backend.updateInfo.error ? root.shell.t("检查更新失败，请稍后重试。", "Update check failed. Try again later.", "更新の確認に失敗しました。後で再試行してください。") : backend.updateInfo.latestVersion ? (backend.updateInfo.hasUpdate ? root.shell.t("新版本：", "New version: ", "新しいバージョン：") : root.shell.t("当前已是最新版本。", "You are up to date. ", "最新バージョンです。")) + (backend.updateInfo.hasUpdate ? backend.updateInfo.latestVersion : "") : "" }
                GlassButton { visible: !!backend.updateInfo.hasUpdate; text: root.shell.t("查看发布页", "View release", "リリースを表示"); dark: root.shell.dark; onClicked: backend.openUrl(backend.updateInfo.releaseURL) }
            }
        }
        ContentCard {
            Layout.fillWidth: true
            implicitHeight: sponsors.implicitHeight + 48
            dark: root.shell.dark
            ColumnLayout {
                id: sponsors
                anchors { left: parent.left; right: parent.right; top: parent.top; margins: 24 }
                spacing: 16
                Text { text: root.shell.t("合作伙伴", "Partners", "パートナー"); color: root.shell.ink; font { pixelSize: 17; weight: Font.DemiBold } }
                Text { text: "ProxyLane"; color: root.shell.ink; font { pixelSize: 16; weight: Font.DemiBold } }
                Text { text: root.shell.t("住宅代理覆盖 195 个国家，95%+ IP 获低风险评级。中文界面，支持 USDT，首次购买优惠码 KIROX30 享 7 折。", "Residential proxies across 195 countries. Chinese interface and USDT payments. Use KIROX30 for 30% off your first purchase.", "195 か国の住宅プロキシ。中国語 UI、USDT 決済。初回購入は KIROX30 で 30% オフ。"); wrapMode: Text.Wrap; Layout.fillWidth: true; color: root.shell.muted; font.pixelSize: 13 }
                GlassButton { text: root.shell.t("访问 ProxyLane", "Visit ProxyLane", "ProxyLane へ"); dark: root.shell.dark; onClicked: backend.openUrl("https://proxylane.dev/?utm_source=kirox&utm_medium=partnership&utm_campaign=kirox_sponsor_202610&utm_content=desktop") }
                Rectangle { height: 1; Layout.fillWidth: true; color: root.shell.dark ? "#364155" : "#e4e9f2" }
                Text { text: "IPWO"; color: root.shell.ink; font { pixelSize: 16; weight: Font.DemiBold } }
                Text { text: root.shell.t("覆盖 195+ 国家和地区的住宅代理。专属折扣码：0205。", "Residential proxies in 195+ countries and regions. Discount code: 0205.", "195 以上の国・地域の住宅プロキシ。割引コード：0205。"); color: root.shell.muted; font.pixelSize: 13; Layout.fillWidth: true; wrapMode: Text.Wrap }
                GlassButton { text: root.shell.t("访问 IPWO", "Visit IPWO", "IPWO へ"); dark: root.shell.dark; onClicked: backend.openUrl("https://www.ipwo.net/?ref=githubKiroX") }
            }
        }
    }
}
