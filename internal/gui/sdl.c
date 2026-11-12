// C helpers for ng's SDL3 window. Kept in C because SDL_Event is a union
// and several SDL macros are not usable from cgo directly.
#include <string.h>
#include "sdl.h"



void ng_flatten(const SDL_Event *e, ng_event *o) {
	memset(o, 0, sizeof(*o));
	o->type = e->type;
	switch (e->type) {
	case SDL_EVENT_KEY_DOWN:
	case SDL_EVENT_KEY_UP:
		o->key = e->key.key;
		o->mod = e->key.mod;
		o->repeat = e->key.repeat;
		break;
	case SDL_EVENT_TEXT_INPUT:
		if (e->text.text)
			SDL_strlcpy(o->text, e->text.text, sizeof(o->text));
		break;
	case SDL_EVENT_MOUSE_BUTTON_DOWN:
	case SDL_EVENT_MOUSE_BUTTON_UP:
		o->x = e->button.x;
		o->y = e->button.y;
		o->button = e->button.button;
		break;
	case SDL_EVENT_MOUSE_MOTION:
		o->x = e->motion.x;
		o->y = e->motion.y;
		o->state = e->motion.state;
		break;
	case SDL_EVENT_MOUSE_WHEEL:
		o->x = e->wheel.x;
		o->y = e->wheel.y;
		if (e->wheel.direction == SDL_MOUSEWHEEL_FLIPPED) {
			o->x = -o->x;
			o->y = -o->y;
		}
		break;
	}
}

// Drop events carry a file name that may be longer than ng_event.text.
char *ng_drop_data(const SDL_Event *e) {
	return e->drop.data ? SDL_strdup(e->drop.data) : NULL;
}

static Uint32 ng_wake_type = 0;

bool ng_init(void) {
	// The identifier becomes the Wayland app_id and the X11 WM_CLASS.
	SDL_SetAppMetadata("ng", NULL, "ng");
	if (!SDL_Init(SDL_INIT_VIDEO))
		return false;
	ng_wake_type = SDL_RegisterEvents(1);
	return ng_wake_type != 0;
}

// ng_wake may be called from any thread.
void ng_wake(void) {
	SDL_Event e;
	SDL_zero(e);
	e.type = ng_wake_type;
	SDL_PushEvent(&e);
}

Uint32 ng_get_wake_type(void) { return ng_wake_type; }

int ng_wait(SDL_Event *e) { return SDL_WaitEvent(e) ? 1 : 0; }
int ng_poll(SDL_Event *e) { return SDL_PollEvent(e) ? 1 : 0; }

SDL_Window *ng_create_window(const char *title, int w, int h) {
	return SDL_CreateWindow(title, w, h,
		SDL_WINDOW_RESIZABLE | SDL_WINDOW_HIGH_PIXEL_DENSITY | SDL_WINDOW_HIDDEN);
}

void ng_fill(SDL_Renderer *r, int x, int y, int w, int h, Uint32 rgb) {
	SDL_SetRenderDrawColor(r, (rgb >> 16) & 0xff, (rgb >> 8) & 0xff, rgb & 0xff, 0xff);
	SDL_FRect rect = { (float)x, (float)y, (float)w, (float)h };
	SDL_RenderFillRect(r, &rect);
}

SDL_Texture *ng_create_texture(SDL_Renderer *r, int access, int w, int h) {
	SDL_Texture *t = SDL_CreateTexture(r, SDL_PIXELFORMAT_RGBA32, access, w, h);
	if (t) {
		SDL_SetTextureBlendMode(t, SDL_BLENDMODE_BLEND);
		SDL_SetTextureScaleMode(t, SDL_SCALEMODE_NEAREST);
	}
	return t;
}

bool ng_update(SDL_Texture *t, int x, int y, int w, int h, const void *pix, int pitch) {
	SDL_Rect rect = { x, y, w, h };
	return SDL_UpdateTexture(t, &rect, pix, pitch);
}

bool ng_blit(SDL_Renderer *r, SDL_Texture *t, int w, int h) {
	SDL_FRect rect = { 0, 0, (float)w, (float)h };
	SDL_SetTextureBlendMode(t, SDL_BLENDMODE_NONE);
	bool ok = SDL_RenderTexture(r, t, &rect, &rect);
	SDL_SetTextureBlendMode(t, SDL_BLENDMODE_BLEND);
	return ok;
}

float ng_pixel_density(SDL_Window *w) {
	float d = SDL_GetWindowPixelDensity(w);
	return d > 0 ? d : 1.0f;
}
