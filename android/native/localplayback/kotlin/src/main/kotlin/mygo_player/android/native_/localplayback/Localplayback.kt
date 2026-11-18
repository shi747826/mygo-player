package mygo_player.android.native_.localplayback

import android.graphics.BitmapFactory
import android.os.Build
import android.os.Looper
import android.view.View
import android.view.ViewGroup
import android.webkit.WebSettings
import android.webkit.WebView
import dev.mygo.NativeException
import dev.mygo.PluginHost
import java.io.File
import java.io.FileOutputStream
import java.net.URI

fun makeController(): Controller = LocalPlaybackController()

private class LocalPlaybackController : Controller {
    override fun enableLocalPlayback(): Long {
        // The Go interface marks this method //mygo:mainthread. Fail closed if
        // a different host ever ignores that annotation.
        check(Looper.myLooper() == Looper.getMainLooper()) {
            "Local playback must be configured on Android's main thread"
        }
        val root = PluginHost.activity?.window?.decorView
            ?: throw NativeException("LocalPlayback", 1, "No active Android Activity")
        var configured = 0L
        fun visit(view: View) {
            if (view is WebView) {
                if (isAppPage(view.url)) {
                    view.settings.mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
                    configured++
                } else {
                    // In particular, reentry after navigation must not leave a
                    // previously configured WebView relaxed on an external URL.
                    view.settings.mixedContentMode = WebSettings.MIXED_CONTENT_NEVER_ALLOW
                }
            }
            if (view is ViewGroup) {
                for (index in 0 until view.childCount) visit(view.getChildAt(index))
            }
        }
        visit(root)
        if (configured == 0L) {
            throw NativeException("LocalPlayback", 2, "No loaded https://mygo.localhost app WebView")
        }
        return configured
    }

    override fun convertCover(path: String): String {
        // Android decodes HEIF from 9.0 on; older devices simply cannot show
        // those covers, so the caller falls back to the placeholder.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) {
            throw NativeException("LocalPlayback", 3, "HEIF decoding needs Android 9")
        }
        val source = File(path)
        val bitmap = BitmapFactory.decodeFile(source.absolutePath)
            ?: throw NativeException("LocalPlayback", 4, "Unsupported image format")
        val target = File(source.absolutePath + ".jpg")
        try {
            FileOutputStream(target).use { out ->
                if (!bitmap.compress(android.graphics.Bitmap.CompressFormat.JPEG, 86, out)) {
                    throw NativeException("LocalPlayback", 5, "Cannot encode cover")
                }
            }
        } finally {
            bitmap.recycle()
        }
        return target.absolutePath
    }
}

// Keep this allowlist in sync with the app scheme. Do not accept suffixes,
// userinfo, HTTP, arbitrary ports, or an origin supplied by page JavaScript.
private fun isAppPage(url: String?): Boolean {
    if (url == null) return false
    return runCatching {
        val uri = URI(url)
        uri.scheme.equals("https", ignoreCase = true) &&
            uri.host.equals("mygo.localhost", ignoreCase = true) &&
            uri.rawUserInfo == null && (uri.port == -1 || uri.port == 443)
    }.getOrDefault(false)
}
