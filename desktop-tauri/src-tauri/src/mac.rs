#![allow(unexpected_cfgs)]
#[cfg(target_os = "macos")]
pub mod window {
    use cocoa::appkit::{NSView, NSWindow, NSWindowButton};
    use cocoa::base::id;
    use cocoa::foundation::NSRect;
    use objc::{msg_send, sel, sel_impl};
    use tauri::WebviewWindow;

    pub fn position_traffic_lights(window: &WebviewWindow) {
        if let Ok(handle) = window.ns_window() {
            let ns_window = handle as id;
            unsafe {
                let close = ns_window.standardWindowButton_(NSWindowButton::NSWindowCloseButton);
                let miniaturize = ns_window.standardWindowButton_(NSWindowButton::NSWindowMiniaturizeButton);
                let zoom = ns_window.standardWindowButton_(NSWindowButton::NSWindowZoomButton);

                if close == cocoa::base::nil || miniaturize == cocoa::base::nil || zoom == cocoa::base::nil {
                    return;
                }

                let title_bar_view: id = msg_send![close, superview];
                if title_bar_view == cocoa::base::nil {
                    return;
                }
                let title_bar_container_view: id = msg_send![title_bar_view, superview];
                if title_bar_container_view == cocoa::base::nil {
                    return;
                }

                let header_height: f64 = 68.0;
                let button_height: f64 = 14.0;
                let x_start: f64 = 18.0;
                let space_between: f64 = 20.0;
                let target_y: f64 = (header_height - button_height) / 2.0;

                let window_frame: NSRect = NSView::frame(ns_window);
                let mut title_bar_rect: NSRect = NSView::frame(title_bar_container_view);
                title_bar_rect.size.height = header_height;
                title_bar_rect.origin.y = window_frame.size.height - header_height;
                let _: () = msg_send![title_bar_container_view, setFrame: title_bar_rect];

                let buttons = [close, miniaturize, zoom];
                for (i, &button) in buttons.iter().enumerate() {
                    let mut rect: NSRect = NSView::frame(button);
                    rect.origin.x = x_start + (i as f64 * space_between);
                    rect.origin.y = target_y;
                    let _: () = msg_send![button, setFrame: rect];
                }
            }
        }
    }
}
