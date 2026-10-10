import QtQuick

Canvas {
    id: icon
    property string name: "overview"
    property color ink: "#79859b"
    implicitWidth: 22
    implicitHeight: 22
    onNameChanged: requestPaint()
    onInkChanged: requestPaint()
    onPaint: {
        const c = getContext("2d");
        c.reset();
        c.strokeStyle = ink;
        c.lineWidth = 1.7;
        c.lineCap = "round";
        c.lineJoin = "round";
        function path(points) {
            c.beginPath();
            c.moveTo(points[0][0], points[0][1]);
            for (let i = 1; i < points.length; i++)
                c.lineTo(points[i][0], points[i][1]);
            c.stroke();
        }
        function circle(x, y, r) {
            c.beginPath();
            c.arc(x, y, r, 0, Math.PI * 2);
            c.stroke();
        }
        if (name === "sun") {
            circle(11, 11, 4);
            for (let i = 0; i < 8; i++) {
                const a = i * Math.PI / 4;
                path([[11 + 7 * Math.cos(a), 11 + 7 * Math.sin(a)], [11 + 9 * Math.cos(a), 11 + 9 * Math.sin(a)]]);
            }
        } else if (name === "moon") {
            c.beginPath();
            c.arc(11, 11, 8, 0.5, 5.3);
            c.arc(15, 7, 6, 3.7, 1.8, true);
            c.closePath();
            c.stroke();
        } else if (name === "accounts" || name === "services") {
            c.strokeRect(3, 5, 16, 12);
            path([[3, 5], [11, 12], [19, 5]]);
        } else if (name === "proxies") {
            circle(11, 11, 8);
            c.beginPath();
            c.ellipse(7, 3, 8, 16);
            c.stroke();
            path([[3, 11], [19, 11]]);
        } else if (name === "logs") {
            path([[7, 5], [19, 5]]);
            path([[7, 11], [19, 11]]);
            path([[7, 17], [16, 17]]);
            for (let y = 5; y < 18; y += 6)
                circle(3, y, 0.6);
        } else if (name === "tasks") {
            path([[5, 3], [17, 3], [17, 19], [5, 19], [5, 3]]);
            path([[8, 8], [10, 10], [14, 6]]);
            path([[8, 14], [14, 14]]);
        } else if (name === "settings") {
            path([[3, 6], [19, 6]]);
            path([[3, 16], [19, 16]]);
            circle(8, 6, 2.5);
            circle(14, 16, 2.5);
        } else if (name === "about") {
            circle(11, 11, 8);
            path([[11, 10], [11, 16]]);
            circle(11, 6, 0.5);
        } else {
            c.strokeRect(3, 3, 6, 6);
            c.strokeRect(13, 3, 6, 6);
            c.strokeRect(3, 13, 6, 6);
            c.strokeRect(13, 13, 6, 6);
        }
    }
}
